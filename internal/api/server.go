package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/buckelew/go-monitor/config"
	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/discord"
	"github.com/buckelew/go-monitor/internal/hub"
	"github.com/buckelew/go-monitor/internal/monitor"
)

// TaskBuilder creates a monitor.Task from a database row.
// Injected to avoid import cycles with platform packages.
type TaskBuilder func(ctx context.Context, queries *database.Queries, dbTask database.Task) (monitor.Task, error)

type Server struct {
	queries       *database.Queries
	db            *sql.DB
	hub           *hub.Hub
	scheduler     *monitor.Scheduler
	bot           *discord.Bot
	cfg           *config.Config
	sessionSecret []byte
	buildTask     TaskBuilder
	mux           *http.ServeMux
}

func NewServer(
	queries *database.Queries,
	db *sql.DB,
	eventHub *hub.Hub,
	scheduler *monitor.Scheduler,
	bot *discord.Bot,
	cfg *config.Config,
	buildTask TaskBuilder,
) *Server {
	s := &Server{
		queries:       queries,
		db:            db,
		hub:           eventHub,
		scheduler:     scheduler,
		bot:           bot,
		cfg:           cfg,
		sessionSecret: []byte(cfg.API.SessionSecret),
		buildTask:     buildTask,
		mux:           http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Auth
	s.mux.HandleFunc("GET /auth/discord", s.handleDiscordAuth)
	s.mux.HandleFunc("GET /auth/discord/callback", s.handleDiscordCallback)
	s.mux.HandleFunc("POST /auth/logout", s.handleLogout)

	// Protected endpoints
	s.mux.HandleFunc("GET /api/me", s.requireAuth(s.handleMe))
	s.mux.HandleFunc("POST /api/servers", s.requireAuth(s.handleCreateServer))
	s.mux.HandleFunc("GET /api/servers", s.requireAuth(s.handleListServers))
	s.mux.HandleFunc("DELETE /api/servers/{id}", s.requireAuth(s.handleDeleteServer))
	s.mux.HandleFunc("GET /api/tasks", s.requireAuth(s.handleListTasks))
	s.mux.HandleFunc("POST /api/subscriptions", s.requireAuth(s.handleCreateSubscription))
	s.mux.HandleFunc("GET /api/subscriptions", s.requireAuth(s.handleListSubscriptions))
	s.mux.HandleFunc("DELETE /api/subscriptions/{id}", s.requireAuth(s.handleDeleteSubscription))
	s.mux.HandleFunc("GET /ws", s.requireAuth(s.handleWebSocket))
}
