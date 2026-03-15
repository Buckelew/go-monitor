package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
)

const (
	sessionCookieName = "session"
	sessionDuration   = 7 * 24 * time.Hour
	stateCookieName   = "oauth_state"
)

// handleDiscordAuth redirects the user to Discord's OAuth2 authorization page.
func (s *Server) handleDiscordAuth(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	params := url.Values{
		"client_id":     {s.cfg.Discord.ClientID},
		"redirect_uri":  {s.cfg.Discord.RedirectURL},
		"response_type": {"code"},
		"scope":         {"identify guilds"},
		"state":         {state},
	}

	http.Redirect(w, r, "https://discord.com/api/oauth2/authorize?"+params.Encode(), http.StatusTemporaryRedirect)
}

// handleDiscordCallback handles the OAuth2 callback from Discord.
func (s *Server) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	tokenResp, err := s.exchangeCode(code)
	if err != nil {
		log.Printf("[auth] token exchange failed: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Fetch Discord user
	discordUser, err := s.fetchDiscordUser(tokenResp.AccessToken)
	if err != nil {
		log.Printf("[auth] failed to fetch user: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Upsert user in database
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	dbUser, err := s.queries.UpsertDiscordUser(r.Context(), database.UpsertDiscordUserParams{
		DiscordUserID:  discordUser.ID,
		Username:       discordUser.Username,
		Avatar:         discordUser.Avatar,
		AccessToken:    tokenResp.AccessToken,
		RefreshToken:   tokenResp.RefreshToken,
		TokenExpiresAt: sql.NullTime{Time: expiresAt, Valid: true},
	})
	if err != nil {
		log.Printf("[auth] failed to upsert user: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionID, err := randomHex(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.queries.CreateSession(r.Context(), database.CreateSessionParams{
		ID:            sessionID,
		DiscordUserID: dbUser.ID,
		ExpiresAt:     time.Now().Add(sessionDuration),
	}); err != nil {
		log.Printf("[auth] failed to create session: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Set signed session cookie
	sig := s.signSession(sessionID)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID + "." + sig,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout deletes the session and clears the cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := s.extractSessionID(r)
	if sessionID != "" {
		s.queries.DeleteSession(r.Context(), sessionID)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type discordUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

func (s *Server) exchangeCode(code string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":     {s.cfg.Discord.ClientID},
		"client_secret": {s.cfg.Discord.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.cfg.Discord.RedirectURL},
	}

	resp, err := http.Post("https://discord.com/api/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange returned %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokenResp, nil
}

func (s *Server) fetchDiscordUser(accessToken string) (*discordUserResponse, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch user returned %d", resp.StatusCode)
	}

	var user discordUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &user, nil
}

func (s *Server) signSession(sessionID string) string {
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) extractSessionID(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return ""
	}

	sessionID, sig := parts[0], parts[1]
	expectedSig := s.signSession(sessionID)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return ""
	}

	return sessionID
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type contextKey string

const userContextKey contextKey = "user"

func userFromContext(ctx context.Context) (database.DiscordUser, bool) {
	u, ok := ctx.Value(userContextKey).(database.DiscordUser)
	return u, ok
}
