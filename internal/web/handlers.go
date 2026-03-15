package web

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
)

var platforms = []string{"shopify", "bigcartel", "squarespace", "reddit"}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt32(s string) sql.NullInt32 {
	if s == "" {
		return sql.NullInt32{}
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(v), Valid: true}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/tasks", http.StatusSeeOther)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	urlSearch := r.URL.Query().Get("url")

	tasks, err := s.queries.GetTasksWithStats(r.Context(), database.GetTasksWithStatsParams{
		Platform:  nullStr(platform),
		UrlSearch: nullStr(urlSearch),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Tasks":      tasks,
		"ActivePage": "tasks",
		"Platforms":  platforms,
		"Filter": map[string]string{
			"Platform": platform,
			"URL":      urlSearch,
		},
	}

	if err := s.templates["tasks"].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

const logsPageSize int32 = 50

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	platform := q.Get("platform")
	status := q.Get("status")
	taskID := q.Get("task_id")
	cursorStr := q.Get("cursor")

	var cursor sql.NullTime
	if cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursor = sql.NullTime{Time: t, Valid: true}
		}
	}

	runs, err := s.queries.GetRecentTaskRuns(r.Context(), database.GetRecentTaskRunsParams{
		Platform:     nullStr(platform),
		FilterStatus: nullStr(status),
		TaskID:       nullInt32(taskID),
		Cursor:       cursor,
		PageLimit:    logsPageSize,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For the task dropdown, fetch all tasks
	allTasks, _ := s.queries.GetTasks(r.Context())

	// Compute next cursor from last row
	var nextCursor string
	if int32(len(runs)) == logsPageSize {
		last := runs[len(runs)-1]
		if last.CompletedAt.Valid {
			nextCursor = last.CompletedAt.Time.Format(time.RFC3339Nano)
		}
	}

	// HTMX infinite scroll: return just rows partial
	if r.Header.Get("HX-Request") == "true" && cursorStr != "" {
		data := map[string]any{
			"Runs":       runs,
			"NextCursor": nextCursor,
			"Filter": map[string]string{
				"Platform": platform,
				"Status":   status,
				"TaskID":   taskID,
			},
		}
		if err := s.templates["logs"].ExecuteTemplate(w, "log-rows", data); err != nil {
			log.Printf("template error: %v", err)
		}
		return
	}

	data := map[string]any{
		"Runs":       runs,
		"ActivePage": "logs",
		"Platforms":  platforms,
		"AllTasks":   allTasks,
		"NextCursor": nextCursor,
		"Filter": map[string]string{
			"Platform": platform,
			"Status":   status,
			"TaskID":   taskID,
		},
	}

	if err := s.templates["logs"].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) handleProxies(w http.ResponseWriter, r *http.Request) {
	lists, err := s.queries.GetProxyLists(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"ProxyLists": lists,
		"ActivePage": "proxies",
	}

	if err := s.templates["proxies"].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) handleCreateProxyList(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	proxiesRaw := r.FormValue("proxies")

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	platform := strings.TrimSpace(r.FormValue("platform"))
	list, err := qtx.CreateProxyList(r.Context(), database.CreateProxyListParams{
		Name:     name,
		Platform: nullStr(platform),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := insertProxiesFromText(r.Context(), qtx, list.ID, proxiesRaw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/proxies", http.StatusSeeOther)
}

func (s *Server) handleEditProxyListForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	lists, err := s.queries.GetProxyLists(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var list *database.GetProxyListsRow
	for _, l := range lists {
		if l.ID == int32(id) {
			list = &l
			break
		}
	}
	if list == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	proxies, err := s.queries.GetProxiesByListID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var lines []string
	for _, p := range proxies {
		if p.Username.Valid && p.Username.String != "" {
			lines = append(lines, p.Host+":"+p.Port+":"+p.Username.String+":"+p.Password.String)
		} else {
			lines = append(lines, p.Host+":"+p.Port)
		}
	}

	data := map[string]any{
		"List":        list,
		"ProxiesText": strings.Join(lines, "\n"),
	}

	if err := s.templates["proxies"].ExecuteTemplate(w, "edit-modal", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) handleUpdateProxyList(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	proxiesRaw := r.FormValue("proxies")

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	if err := qtx.UpdateProxyListName(r.Context(), database.UpdateProxyListNameParams{
		ID:   int32(id),
		Name: name,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := qtx.DeleteProxiesByListID(r.Context(), int32(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := insertProxiesFromText(r.Context(), qtx, int32(id), proxiesRaw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteProxyList(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.queries.DeleteProxiesByListID(r.Context(), int32(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.queries.DeleteProxyList(r.Context(), int32(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func insertProxiesFromText(ctx context.Context, qtx *database.Queries, listID int32, raw string) error {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 2 {
			continue
		}

		params := database.CreateProxyParams{
			ProxyListID: listID,
			Host:        parts[0],
			Port:        parts[1],
		}

		if len(parts) >= 4 {
			params.Username = sql.NullString{String: parts[2], Valid: true}
			params.Password = sql.NullString{String: parts[3], Valid: true}
		}

		if err := qtx.CreateProxy(ctx, params); err != nil {
			return err
		}
	}
	return nil
}
