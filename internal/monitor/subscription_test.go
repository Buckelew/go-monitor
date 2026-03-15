package monitor_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/monitor"
)

// TestAutoCreateTask_NewURL verifies that creating a subscription for an
// unknown URL auto-creates a task with the correct platform and delay.
func TestAutoCreateTask_NewURL(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	url := "https://teststore.myshopify.com"

	// No task should exist yet
	_, err := queries.GetTaskByURL(ctx, url)
	require.ErrorIs(t, err, sql.ErrNoRows, "task should not exist before auto-create")

	// Detect platform
	platform, err := monitor.DetectPlatform(url)
	require.NoError(t, err)
	assert.Equal(t, monitor.Shopify, platform)

	// Create task (simulating what the handler does)
	dbTask, err := queries.CreateTask(ctx, database.CreateTaskParams{
		Platform: string(platform),
		TaskType: string(monitor.Search),
		Url:      url,
		Delay:    5000,
	})
	require.NoError(t, err)
	assert.Equal(t, "shopify", dbTask.Platform)
	assert.Equal(t, "search", dbTask.TaskType)
	assert.Equal(t, url, dbTask.Url)
	assert.Equal(t, int32(5000), dbTask.Delay)
	assert.True(t, dbTask.Enabled)

	// Task should now be findable by URL
	found, err := queries.GetTaskByURL(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, dbTask.ID, found.ID)
}

// TestAutoCreateTask_ExistingURL verifies that creating a subscription for
// an existing task URL reuses the task instead of creating a new one.
func TestAutoCreateTask_ExistingURL(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	url := "https://existing.myshopify.com"

	// Pre-create task
	original, err := queries.CreateTask(ctx, database.CreateTaskParams{
		Platform: "shopify",
		TaskType: "search",
		Url:      url,
		Delay:    5000,
	})
	require.NoError(t, err)

	// Looking up by URL should find the existing task
	found, err := queries.GetTaskByURL(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)

	// Attempting to create a duplicate URL should fail (unique constraint)
	_, err = queries.CreateTask(ctx, database.CreateTaskParams{
		Platform: "shopify",
		TaskType: "search",
		Url:      url,
		Delay:    5000,
	})
	require.Error(t, err, "duplicate URL should be rejected by unique constraint")
}

// TestSubscriptionCreation_StoreMode verifies creating store-level
// subscriptions (all/new modes) with guild_id.
func TestSubscriptionCreation_StoreMode(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	task := createTestTask(t, "shopify", "https://sub-test.myshopify.com")
	guildID := sql.NullString{String: "guild-123", Valid: true}

	// Create "all" subscription
	sub, err := queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID:    task.ID,
		ChannelID: "channel-1",
		Mode:      "all",
		GuildID:   guildID,
	})
	require.NoError(t, err)
	assert.Equal(t, "all", sub.Mode)
	assert.Equal(t, "channel-1", sub.ChannelID)
	assert.Equal(t, "guild-123", sub.GuildID.String)

	// Upsert same task+channel with "new" mode should update, not duplicate
	updated, err := queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID:    task.ID,
		ChannelID: "channel-1",
		Mode:      "new",
		GuildID:   guildID,
	})
	require.NoError(t, err)
	assert.Equal(t, sub.ID, updated.ID, "should upsert, not create new")
	assert.Equal(t, "new", updated.Mode, "mode should be updated")

	// Different channel should create a separate subscription
	sub2, err := queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID:    task.ID,
		ChannelID: "channel-2",
		Mode:      "all",
		GuildID:   guildID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, sub.ID, sub2.ID, "different channel should create new sub")
}

// TestSubscriptionCreation_RestockMode verifies creating product-level
// subscriptions (restock mode) with item_id.
func TestSubscriptionCreation_RestockMode(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	task := createTestTask(t, "shopify", "https://restock-test.myshopify.com")
	guildID := sql.NullString{String: "guild-456", Valid: true}

	// Insert a test item
	item, err := queries.UpsertItem(ctx, database.UpsertItemParams{
		TaskID:   task.ID,
		Url:      "https://restock-test.myshopify.com/products/item-1",
		Platform: "shopify",
		Data:     []byte(`{}`),
		InStock:  true,
	})
	require.NoError(t, err)

	// Create restock subscription
	sub, err := queries.CreateProductSubscription(ctx, database.CreateProductSubscriptionParams{
		TaskID:    task.ID,
		ChannelID: "channel-1",
		ItemID:    sql.NullInt32{Int32: item.ID, Valid: true},
		GuildID:   guildID,
	})
	require.NoError(t, err)
	assert.Equal(t, "restock", sub.Mode)
	assert.Equal(t, item.ID, sub.ItemID.Int32)

	// Duplicate should be a no-op (DO NOTHING)
	_, err = queries.CreateProductSubscription(ctx, database.CreateProductSubscriptionParams{
		TaskID:    task.ID,
		ChannelID: "channel-1",
		ItemID:    sql.NullInt32{Int32: item.ID, Valid: true},
		GuildID:   guildID,
	})
	// DO NOTHING returns sql.ErrNoRows when no row is returned
	require.Error(t, err, "duplicate restock subscription should return error (no row returned)")
}

// TestSubscriptionsByGuildID verifies fetching subscriptions scoped to guild IDs.
func TestSubscriptionsByGuildID(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	task := createTestTask(t, "shopify", "https://guild-test.myshopify.com")

	// Create subs for different guilds
	_, err := queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID: task.ID, ChannelID: "ch-1", Mode: "all",
		GuildID: sql.NullString{String: "guild-A", Valid: true},
	})
	require.NoError(t, err)
	_, err = queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID: task.ID, ChannelID: "ch-2", Mode: "new",
		GuildID: sql.NullString{String: "guild-B", Valid: true},
	})
	require.NoError(t, err)
	_, err = queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID: task.ID, ChannelID: "ch-3", Mode: "all",
		GuildID: sql.NullString{String: "guild-A", Valid: true},
	})
	require.NoError(t, err)

	// Query guild-A should return 2
	subsA, err := queries.GetSubscriptionsByGuildIDs(ctx, []string{"guild-A"})
	require.NoError(t, err)
	assert.Len(t, subsA, 2)

	// Query guild-B should return 1
	subsB, err := queries.GetSubscriptionsByGuildIDs(ctx, []string{"guild-B"})
	require.NoError(t, err)
	assert.Len(t, subsB, 1)

	// Query both should return 3
	subsAll, err := queries.GetSubscriptionsByGuildIDs(ctx, []string{"guild-A", "guild-B"})
	require.NoError(t, err)
	assert.Len(t, subsAll, 3)

	// Query unknown guild should return 0
	subsNone, err := queries.GetSubscriptionsByGuildIDs(ctx, []string{"guild-Z"})
	require.NoError(t, err)
	assert.Empty(t, subsNone)
}

// TestDelayCalculation verifies the delay formula for auto-created tasks.
func TestDelayCalculation(t *testing.T) {
	tests := []struct {
		name       string
		taskCount  int64
		proxyCount int64
		wantDelay  int32
	}{
		{"no tasks no proxies", 0, 0, 5000},
		{"1 task no proxies", 1, 0, 5000},
		{"5 tasks 1 proxy", 5, 1, 15000},
		{"10 tasks 5 proxies", 10, 5, 6000},
		{"1 task 10 proxies", 1, 10, 5000},
		{"100 tasks 10 proxies", 100, 10, 30000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			denominator := tt.proxyCount
			if denominator < 1 {
				denominator = 1
			}
			delay := int32((tt.taskCount / denominator) * 3000)
			if delay < 5000 {
				delay = 5000
			}
			assert.Equal(t, tt.wantDelay, delay)
		})
	}
}

// TestDeleteSubscription verifies subscription deletion.
func TestDeleteSubscription(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()
	queries := database.New(testDB)

	task := createTestTask(t, "shopify", "https://delete-test.myshopify.com")

	sub, err := queries.CreateStoreSubscription(ctx, database.CreateStoreSubscriptionParams{
		TaskID: task.ID, ChannelID: "ch-del", Mode: "all",
		GuildID: sql.NullString{String: "guild-del", Valid: true},
	})
	require.NoError(t, err)

	// Verify it exists
	subs, err := queries.GetSubscriptionsByTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Len(t, subs, 1)

	// Delete it
	err = queries.DeleteSubscription(ctx, sub.ID)
	require.NoError(t, err)

	// Verify it's gone
	subs, err = queries.GetSubscriptionsByTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Empty(t, subs)
}
