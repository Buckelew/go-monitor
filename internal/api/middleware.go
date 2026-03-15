package api

import (
	"context"
	"net/http"
	"time"
)

// requireAuth wraps a handler function and ensures the request has a valid session.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := s.extractSessionID(r)
		if sessionID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		session, err := s.queries.GetSession(r.Context(), sessionID)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if session.ExpiresAt.Before(time.Now()) {
			s.queries.DeleteSession(r.Context(), sessionID)
			http.Error(w, "session expired", http.StatusUnauthorized)
			return
		}

		user, err := s.queries.GetDiscordUserByID(r.Context(), session.DiscordUserID)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}
