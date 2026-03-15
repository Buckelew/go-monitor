package web

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
)

//go:embed templates static
var content embed.FS

type Server struct {
	queries   *database.Queries
	db        *sql.DB
	templates map[string]*template.Template
	mux       *http.ServeMux
}

func NewServer(queries *database.Queries, db *sql.DB) *Server {
	funcMap := template.FuncMap{
		"formatDelay":    formatDelay,
		"timeAgo":        timeAgo,
		"formatDuration": formatDuration,
		"formatTime":     formatTime,
	}

	pages := []string{"tasks", "logs", "proxies"}
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
		queries:   queries,
		db:        db,
		templates: templates,
		mux:       http.NewServeMux(),
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.FileServerFS(content))
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /tasks", s.handleTasks)
	s.mux.HandleFunc("GET /logs", s.handleLogs)
	s.mux.HandleFunc("GET /proxies", s.handleProxies)
	s.mux.HandleFunc("POST /proxies", s.handleCreateProxyList)
	s.mux.HandleFunc("GET /proxies/{id}/edit", s.handleEditProxyListForm)
	s.mux.HandleFunc("PUT /proxies/{id}", s.handleUpdateProxyList)
	s.mux.HandleFunc("DELETE /proxies/{id}", s.handleDeleteProxyList)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

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
