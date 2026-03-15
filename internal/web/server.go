package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/discord"
)

//go:embed templates static
var content embed.FS

type contextKey string

const userContextKey contextKey = "user"

type Server struct {
	queries       *database.Queries
	db            *sql.DB
	bot           *discord.Bot
	clientID      string
	sessionSecret []byte
	templates     map[string]*template.Template
	mux           *http.ServeMux
}

func NewServer(queries *database.Queries, db *sql.DB, sessionSecret string, bot *discord.Bot, clientID string) *Server {
	funcMap := template.FuncMap{
		"formatDelay":    formatDelay,
		"timeAgo":        timeAgo,
		"formatDuration": formatDuration,
		"formatTime":     formatTime,
	}

	pages := []string{"tasks", "logs", "proxies", "dashboard"}
	templates := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		templates[page] = template.Must(
			template.New("").Funcs(funcMap).ParseFS(content,
				"templates/layout.html",
				"templates/"+page+".html",
			),
		)
	}

	s := &Server{
		queries:       queries,
		db:            db,
		bot:           bot,
		clientID:      clientID,
		sessionSecret: []byte(sessionSecret),
		templates:     templates,
		mux:           http.NewServeMux(),
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.FileServerFS(content))

	// Redirect root
	s.mux.HandleFunc("GET /{$}", s.handleIndex)

	// User dashboard (any authenticated user)
	s.mux.HandleFunc("GET /dashboard", s.requireAuth(s.handleDashboard))

	// User dashboard actions
	s.mux.HandleFunc("GET /dashboard/detect-platform", s.requireAuth(s.handleDetectPlatform))
	s.mux.HandleFunc("POST /dashboard/subscriptions", s.requireAuth(s.handleCreateSubscription))
	s.mux.HandleFunc("DELETE /dashboard/subscriptions/{id}", s.requireAuth(s.handleDeleteSubscription))

	// Admin routes
	s.mux.HandleFunc("GET /tasks", s.requireAdmin(s.handleTasks))
	s.mux.HandleFunc("GET /logs", s.requireAdmin(s.handleLogs))
	s.mux.HandleFunc("GET /proxies", s.requireAdmin(s.handleProxies))
	s.mux.HandleFunc("POST /proxies", s.requireAdmin(s.handleCreateProxyList))
	s.mux.HandleFunc("GET /proxies/{id}/edit", s.requireAdmin(s.handleEditProxyListForm))
	s.mux.HandleFunc("PUT /proxies/{id}", s.requireAdmin(s.handleUpdateProxyList))
	s.mux.HandleFunc("DELETE /proxies/{id}", s.requireAdmin(s.handleDeleteProxyList))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// --- auth middleware ---

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticateRequest(r)
		if !ok {
			http.Redirect(w, r, "/auth/discord", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticateRequest(r)
		if !ok {
			http.Redirect(w, r, "/auth/discord", http.StatusSeeOther)
			return
		}
		if user.Role != "admin" {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) authenticateRequest(r *http.Request) (database.DiscordUser, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return database.DiscordUser{}, false
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return database.DiscordUser{}, false
	}

	sessionID, sig := parts[0], parts[1]

	// Verify HMAC signature
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(sessionID))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return database.DiscordUser{}, false
	}

	session, err := s.queries.GetSession(r.Context(), sessionID)
	if err != nil {
		return database.DiscordUser{}, false
	}
	if session.ExpiresAt.Before(time.Now()) {
		s.queries.DeleteSession(r.Context(), sessionID)
		return database.DiscordUser{}, false
	}

	user, err := s.queries.GetDiscordUserByID(r.Context(), session.DiscordUserID)
	if err != nil {
		return database.DiscordUser{}, false
	}

	return user, true
}

func userFromContext(ctx context.Context) (database.DiscordUser, bool) {
	u, ok := ctx.Value(userContextKey).(database.DiscordUser)
	return u, ok
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticateRequest(r)
	if !ok {
		http.Redirect(w, r, "/auth/discord", http.StatusSeeOther)
		return
	}
	if user.Role == "admin" {
		http.Redirect(w, r, "/tasks", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// --- template helpers ---

func formatDelay(ms int32) string {
	if ms >= 60000 {
		return fmt.Sprintf("%dm", ms/60000)
	}
	if ms >= 1000 {
		return fmt.Sprintf("%.0fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

func timeAgo(t sql.NullTime) string {
	if !t.Valid {
		return "never"
	}
	d := time.Since(t.Time)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func formatDuration(seconds float64) string {
	if seconds == 0 {
		return "-"
	}
	if seconds < 1 {
		return fmt.Sprintf("%dms", int(math.Round(seconds*1000)))
	}
	return fmt.Sprintf("%.1fs", seconds)
}

func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return "-"
	}
	return t.Time.Format("Jan 02 15:04:05")
}
