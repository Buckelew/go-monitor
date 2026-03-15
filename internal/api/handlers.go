package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/monitor"
)

// handleMe returns the authenticated user's info.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              user.ID,
		"discord_user_id": user.DiscordUserID,
		"username":        user.Username,
		"avatar":          user.Avatar,
	})
}

// handleCreateServer registers a Discord server for the user.
func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	var req struct {
		GuildID   string `json:"guild_id"`
		GuildName string `json:"guild_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.GuildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}

	// Verify user is in the guild via their OAuth token
	inGuild, err := s.userInGuild(user.AccessToken, req.GuildID)
	if err != nil {
		log.Printf("[api] failed to check user guilds: %v", err)
		http.Error(w, "failed to verify guild membership", http.StatusInternalServerError)
		return
	}
	if !inGuild {
		http.Error(w, "you are not a member of this guild", http.StatusForbidden)
		return
	}

	// Verify bot is in the guild
	_, err = s.bot.Session().Guild(req.GuildID)
	if err != nil {
		http.Error(w, "bot is not in this guild", http.StatusBadRequest)
		return
	}

	server, err := s.queries.CreateUserServer(r.Context(), database.CreateUserServerParams{
		DiscordUserID: user.ID,
		GuildID:       req.GuildID,
		GuildName:     req.GuildName,
	})
	if err != nil {
		log.Printf("[api] failed to create server: %v", err)
		http.Error(w, "failed to register server", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, server)
}

// handleListServers returns the user's registered servers.
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	servers, err := s.queries.GetUserServers(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to list servers", http.StatusInternalServerError)
		return
	}
	if servers == nil {
		servers = []database.UserServer{}
	}
	writeJSON(w, http.StatusOK, servers)
}

// handleDeleteServer removes a registered server.
func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.queries.DeleteUserServer(r.Context(), database.DeleteUserServerParams{
		ID:            int32(id),
		DiscordUserID: user.ID,
	}); err != nil {
		http.Error(w, "failed to delete server", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListTasks returns all tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.queries.GetTasks(r.Context())
	if err != nil {
		http.Error(w, "failed to list tasks", http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []database.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// handleCreateSubscription creates a subscription, auto-creating the task if needed.
func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	var req struct {
		URL       string `json:"url"`
		ChannelID string `json:"channel_id"`
		GuildID   string `json:"guild_id"`
		Mode      string `json:"mode"`
		ItemID    *int32 `json:"item_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.URL == "" || req.ChannelID == "" || req.GuildID == "" || req.Mode == "" {
		http.Error(w, "url, channel_id, guild_id, and mode are required", http.StatusBadRequest)
		return
	}

	// Verify guild belongs to user
	_, err := s.queries.GetUserServerByGuildID(r.Context(), database.GetUserServerByGuildIDParams{
		DiscordUserID: user.ID,
		GuildID:       req.GuildID,
	})
	if err != nil {
		http.Error(w, "guild not registered", http.StatusForbidden)
		return
	}

	// Verify channel belongs to guild via bot
	channels, err := s.bot.Session().GuildChannels(req.GuildID)
	if err != nil {
		log.Printf("[api] failed to get guild channels: %v", err)
		http.Error(w, "failed to verify channel", http.StatusInternalServerError)
		return
	}
	channelValid := false
	for _, ch := range channels {
		if ch.ID == req.ChannelID {
			channelValid = true
			break
		}
	}
	if !channelValid {
		http.Error(w, "channel does not belong to this guild", http.StatusBadRequest)
		return
	}

	// Look up or auto-create task
	dbTask, err := s.queries.GetTaskByURL(r.Context(), req.URL)
	if err == sql.ErrNoRows {
		dbTask, err = s.autoCreateTask(r.Context(), req.URL)
		if err != nil {
			log.Printf("[api] failed to auto-create task: %v", err)
			http.Error(w, fmt.Sprintf("failed to create task: %v", err), http.StatusBadRequest)
			return
		}
	} else if err != nil {
		http.Error(w, "failed to look up task", http.StatusInternalServerError)
		return
	}

	// Create subscription
	guildID := sql.NullString{String: req.GuildID, Valid: true}
	var sub database.TaskSubscription
	switch req.Mode {
	case "all", "new":
		sub, err = s.queries.CreateStoreSubscription(r.Context(), database.CreateStoreSubscriptionParams{
			TaskID:    dbTask.ID,
			ChannelID: req.ChannelID,
			Mode:      req.Mode,
			GuildID:   guildID,
		})
	case "restock":
		if req.ItemID == nil {
			http.Error(w, "item_id is required for restock mode", http.StatusBadRequest)
			return
		}
		sub, err = s.queries.CreateProductSubscription(r.Context(), database.CreateProductSubscriptionParams{
			TaskID:    dbTask.ID,
			ChannelID: req.ChannelID,
			ItemID:    sql.NullInt32{Int32: *req.ItemID, Valid: true},
			GuildID:   guildID,
		})
	default:
		http.Error(w, "mode must be all, new, or restock", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("[api] failed to create subscription: %v", err)
		http.Error(w, "failed to create subscription", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

// handleListSubscriptions returns subscriptions for the user's guilds.
func (s *Server) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	servers, err := s.queries.GetUserServers(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to list servers", http.StatusInternalServerError)
		return
	}

	var guildIDs []string
	for _, srv := range servers {
		guildIDs = append(guildIDs, srv.GuildID)
	}

	if len(guildIDs) == 0 {
		writeJSON(w, http.StatusOK, []database.TaskSubscription{})
		return
	}

	subs, err := s.queries.GetSubscriptionsByGuildIDs(r.Context(), guildIDs)
	if err != nil {
		http.Error(w, "failed to list subscriptions", http.StatusInternalServerError)
		return
	}
	if subs == nil {
		subs = []database.TaskSubscription{}
	}
	writeJSON(w, http.StatusOK, subs)
}

// handleDeleteSubscription removes a subscription.
func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.queries.DeleteSubscription(r.Context(), int32(id)); err != nil {
		http.Error(w, "failed to delete subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func (s *Server) autoCreateTask(ctx context.Context, rawURL string) (database.Task, error) {
	platform, err := monitor.DetectPlatform(rawURL)
	if err != nil {
		return database.Task{}, err
	}

	// Determine task type — Target uses ATC, everything else uses Search.
	taskType := monitor.Search
	if platform == monitor.Target {
		// Validate that we can extract a TCIN from the input.
		if _, err := monitor.ExtractTargetTCIN(rawURL); err != nil {
			return database.Task{}, fmt.Errorf("invalid Target URL: %w", err)
		}
		taskType = monitor.ATC
	}

	taskCount, err := s.queries.CountTasksByPlatform(ctx, string(platform))
	if err != nil {
		return database.Task{}, fmt.Errorf("count tasks: %w", err)
	}

	proxyCount, err := s.queries.CountProxiesByPlatform(ctx, string(platform))
	if err != nil {
		return database.Task{}, fmt.Errorf("count proxies: %w", err)
	}

	denominator := proxyCount
	if denominator < 1 {
		denominator = 1
	}
	delay := int32((taskCount / denominator) * 3000)
	if delay < 5000 {
		delay = 5000
	}

	// Look up proxy list for this platform
	var proxyListID sql.NullInt32
	proxyList, err := s.queries.GetProxyListByPlatform(ctx, string(platform))
	if err == nil {
		proxyListID = sql.NullInt32{Int32: proxyList.ID, Valid: true}
	}

	dbTask, err := s.queries.CreateTask(ctx, database.CreateTaskParams{
		Platform:    string(platform),
		TaskType:    string(taskType),
		Url:         rawURL,
		Delay:       delay,
		ProxyListID: proxyListID,
	})
	if err != nil {
		return database.Task{}, fmt.Errorf("create task: %w", err)
	}

	// Build and add to scheduler
	task, err := s.buildTask(ctx, s.queries, dbTask)
	if err != nil {
		return database.Task{}, fmt.Errorf("build task: %w", err)
	}
	s.scheduler.AddTask(ctx, task)

	return dbTask, nil
}

func (s *Server) userInGuild(accessToken, guildID string) (bool, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("guilds API returned %d", resp.StatusCode)
	}

	var guilds []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return false, err
	}

	for _, g := range guilds {
		if g.ID == guildID {
			return true, nil
		}
	}
	return false, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
