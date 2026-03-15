package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/monitor"
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

type subscriptionView struct {
	Sub          database.TaskSubscription
	TaskURL      string
	TaskPlatform string
	ChannelName  string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	// Auto-sync: fetch user's guilds from Discord, register any where
	// user has MANAGE_GUILD and bot is present
	s.syncUserServers(r.Context(), user)

	servers, err := s.queries.GetUserServers(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get selected server from query param, default to first
	selectedGuildID := r.URL.Query().Get("server")
	var selectedServer *database.UserServer
	for i := range servers {
		if servers[i].GuildID == selectedGuildID {
			selectedServer = &servers[i]
			break
		}
	}
	if selectedServer == nil && len(servers) > 0 {
		selectedServer = &servers[0]
	}

	// Get subscriptions for selected server, enriched with task info
	var subViews []subscriptionView
	if selectedServer != nil {
		subs, err := s.queries.GetSubscriptionsByGuildIDs(r.Context(), []string{selectedServer.GuildID})
		if err != nil {
			log.Printf("failed to get subscriptions: %v", err)
		}
		for _, sub := range subs {
			sv := subscriptionView{Sub: sub, ChannelName: sub.ChannelID}
			task, err := s.queries.GetTask(r.Context(), sub.TaskID)
			if err == nil {
				sv.TaskURL = task.Url
				sv.TaskPlatform = task.Platform
			}
			subViews = append(subViews, sv)
		}
	}

	// Get guild channels for the modal dropdown
	type channelInfo struct {
		ID   string
		Name string
	}
	var channels []channelInfo
	if selectedServer != nil && s.bot != nil {
		guildChannels, err := s.bot.Session().GuildChannels(selectedServer.GuildID)
		if err == nil {
			for _, ch := range guildChannels {
				// Only text channels (type 0)
				if ch.Type == 0 {
					channels = append(channels, channelInfo{ID: ch.ID, Name: "#" + ch.Name})
				}
			}
		}
	}

	data := map[string]any{
		"ActivePage":     "dashboard",
		"User":           user,
		"Servers":        servers,
		"SelectedServer": selectedServer,
		"Subscriptions":  subViews,
		"Channels":       channels,
		"ClientID":       s.clientID,
		"IsAdmin":        user.Role == "admin",
	}

	if err := s.templates["dashboard"].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) handleDetectPlatform(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":"url is required"}`))
		return
	}

	platform, err := monitor.DetectPlatform(rawURL)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}
	w.Write([]byte(`{"platform":"` + string(platform) + `"}`))
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	channelID := r.FormValue("channel_id")
	guildID := r.FormValue("guild_id")
	mode := r.FormValue("mode")

	if url == "" || channelID == "" || guildID == "" || mode == "" {
		http.Error(w, "all fields are required", http.StatusBadRequest)
		return
	}

	// Look up or auto-create task
	dbTask, err := s.queries.GetTaskByURL(r.Context(), url)
	if err != nil {
		// Auto-detect platform and create task
		platform, err := monitor.DetectPlatform(url)
		if err != nil {
			http.Error(w, "unsupported platform: "+err.Error(), http.StatusBadRequest)
			return
		}
		dbTask, err = s.queries.CreateTask(r.Context(), database.CreateTaskParams{
			Platform: string(platform),
			TaskType: "search",
			Url:      url,
			Delay:    5000,
		})
		if err != nil {
			log.Printf("failed to create task: %v", err)
			http.Error(w, "failed to create task", http.StatusInternalServerError)
			return
		}
	}

	// Create subscription
	guildIDNull := sql.NullString{String: guildID, Valid: true}
	switch mode {
	case "all", "new":
		_, err = s.queries.CreateStoreSubscription(r.Context(), database.CreateStoreSubscriptionParams{
			TaskID: dbTask.ID, ChannelID: channelID, Mode: mode, GuildID: guildIDNull,
		})
	default:
		http.Error(w, "invalid mode", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to create subscription: %v", err)
		http.Error(w, "failed to create subscription", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard?server="+guildID, http.StatusSeeOther)
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.queries.DeleteSubscription(r.Context(), int32(id)); err != nil {
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// syncUserServers fetches the user's Discord guilds and auto-registers
// any where the user has MANAGE_GUILD and the bot is present.
func (s *Server) syncUserServers(ctx context.Context, user database.DiscordUser) {
	const manageGuild = 0x20

	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+user.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[sync] failed to fetch guilds for user %d: %v", user.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var guilds []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Permissions int64  `json:"permissions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return
	}

	for _, g := range guilds {
		if g.Permissions&manageGuild == 0 {
			continue
		}
		// Check if bot is in this guild
		_, err := s.bot.Session().Guild(g.ID)
		if err != nil {
			continue
		}
		s.queries.CreateUserServer(ctx, database.CreateUserServerParams{
			DiscordUserID: user.ID,
			GuildID:       g.ID,
			GuildName:     g.Name,
		})
	}
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

	user, _ := userFromContext(r.Context())
	data := map[string]any{
		"Tasks":      tasks,
		"ActivePage": "tasks",
		"Platforms":  platforms,
		"User":       user,
		"IsAdmin":    user.Role == "admin",
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

	user, _ := userFromContext(r.Context())
	data := map[string]any{
		"Runs":       runs,
		"ActivePage": "logs",
		"Platforms":  platforms,
		"AllTasks":   allTasks,
		"NextCursor": nextCursor,
		"User":       user,
		"IsAdmin":    user.Role == "admin",
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

type proxyListView struct {
	database.GetProxyListsRow
	Platforms []string
}

func (s *Server) handleProxies(w http.ResponseWriter, r *http.Request) {
	lists, err := s.queries.GetProxyLists(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []proxyListView
	for _, l := range lists {
		platforms, err := s.queries.GetPlatformsByProxyListID(r.Context(), l.ID)
		if err != nil {
			platforms = nil
		}
		views = append(views, proxyListView{GetProxyListsRow: l, Platforms: platforms})
	}

	user, _ := userFromContext(r.Context())
	data := map[string]any{
		"ProxyLists": views,
		"ActivePage": "proxies",
		"User":       user,
		"IsAdmin":    user.Role == "admin",
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

	list, err := qtx.CreateProxyList(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set platforms (multiple via checkboxes)
	selectedPlatforms := r.Form["platforms"]
	for _, p := range selectedPlatforms {
		qtx.SetProxyListPlatform(r.Context(), database.SetProxyListPlatformParams{
			ProxyListID: list.ID,
			Platform:    p,
		})
	}

	if err := insertProxiesFromText(r.Context(), qtx, list.ID, proxiesRaw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Bump proxy version so running tasks pick up new proxies
	s.proxyRegistry.Bump(list.ID)

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

	listPlatforms, _ := s.queries.GetPlatformsByProxyListID(r.Context(), int32(id))
	platformSet := make(map[string]bool)
	for _, p := range listPlatforms {
		platformSet[p] = true
	}

	data := map[string]any{
		"List":          list,
		"ProxiesText":   strings.Join(lines, "\n"),
		"PlatformSet":   platformSet,
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

	// Update platforms
	qtx.DeleteProxyListPlatforms(r.Context(), int32(id))
	for _, p := range r.Form["platforms"] {
		qtx.SetProxyListPlatform(r.Context(), database.SetProxyListPlatformParams{
			ProxyListID: int32(id),
			Platform:    p,
		})
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Bump proxy version so running tasks pick up changes
	s.proxyRegistry.Bump(int32(id))

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
