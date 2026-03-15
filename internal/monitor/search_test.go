package monitor_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/bigcartel"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/reddit"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/shopify"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/squarespace"
)

func TestE2E_Shopify(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	truncateTables(t)
	task := createTestTask(t, "shopify", server.URL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	// baseURL = server.URL so HTTP requests go to the test server
	// URL = server.URL so product links are constructed from server.URL
	scraper := shopify.NewSearchShopify(server.URL, client)
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	ctx := context.Background()

	products := []shopifyFixture{
		{Handle: "product-a", Title: "Product A", Available: true, ImageSrc: "https://img.test/a.jpg"},
		{Handle: "product-b", Title: "Product B", Available: true, ImageSrc: "https://img.test/b.jpg"},
		{Handle: "product-c", Title: "Product C", Available: true, ImageSrc: "https://img.test/c.jpg"},
	}

	// Run 1: 3 new products, all in stock
	store.SetResponse(makeShopifyResponse(products...))
	result, err := searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Events, 3, "run 1: expected 3 new_product events")
	for _, e := range result.Events {
		assert.Equal(t, monitor.EventNewProduct, e.Type)
		assert.NotZero(t, e.ItemID, "new_product events should carry a DB item ID")
	}
	items := getItems(t, task.ID)
	require.Len(t, items, 3)
	for _, it := range items {
		assert.True(t, it.InStock)
		assert.False(t, it.Delisted)
	}

	// Run 2: Same 3 products — no events
	store.SetResponse(makeShopifyResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 2: expected 0 events")

	// Run 3: Product C goes OOS
	products[2].Available = false
	store.SetResponse(makeShopifyResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 3: OOS transition produces no event")
	items = getItems(t, task.ID)
	for _, it := range items {
		if it.Url == fmt.Sprintf("%s/products/product-c", server.URL) {
			assert.False(t, it.InStock, "product-c should be OOS")
		}
	}

	// Run 4: Product C back in stock — restock event
	products[2].Available = true
	store.SetResponse(makeShopifyResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 4: expected 1 restock event")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)
	assert.NotZero(t, result.Events[0].ItemID, "restock events should carry a DB item ID")
	items = getItems(t, task.ID)
	for _, it := range items {
		assert.True(t, it.InStock)
	}

	// Run 5: Product B removed (delisted)
	remaining := []shopifyFixture{products[0], products[2]}
	store.SetResponse(makeShopifyResponse(remaining...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 5: expected 1 delisted event")
	assert.Equal(t, monitor.EventDelisted, result.Events[0].Type)
	assert.NotZero(t, result.Events[0].ItemID, "delisted events should carry a DB item ID")
	items = getItems(t, task.ID)
	for _, it := range items {
		if it.Url == fmt.Sprintf("%s/products/product-b", server.URL) {
			assert.True(t, it.Delisted, "product-b should be delisted")
		}
	}

	// Run 6: Product B reappears — restock event
	store.SetResponse(makeShopifyResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 6: expected 1 restock event (un-delist)")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)
	assert.NotZero(t, result.Events[0].ItemID, "un-delist restock events should carry a DB item ID")
	items = getItems(t, task.ID)
	for _, it := range items {
		assert.False(t, it.Delisted)
	}
}



func TestE2E_BigCartel(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	truncateTables(t)
	storeURL := "https://testshop.bigcartel.com"
	task := createTestTask(t, "bigcartel", storeURL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := bigcartel.NewSearchBigCartel(storeURL, client, &monitor.Opts{BaseURL: server.URL})
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	ctx := context.Background()

	products := []bigcartelFixture{
		{Name: "Item A", Permalink: "item-a", Status: "active", ImageURL: "https://img.test/a.jpg"},
		{Name: "Item B", Permalink: "item-b", Status: "active", ImageURL: "https://img.test/b.jpg"},
		{Name: "Item C", Permalink: "item-c", Status: "active", ImageURL: "https://img.test/c.jpg"},
	}

	// Run 1: 3 new products
	store.SetResponse(makeBigCartelResponse(products...))
	result, err := searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Events, 3, "run 1: expected 3 new_product events")
	items := getItems(t, task.ID)
	require.Len(t, items, 3)
	for _, it := range items {
		assert.True(t, it.InStock)
		assert.False(t, it.Delisted)
	}

	// Run 2: Same — no events
	store.SetResponse(makeBigCartelResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 2: expected 0 events")

	// Run 3: Item C sold out
	products[2].Status = "sold_out"
	store.SetResponse(makeBigCartelResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 3: OOS transition produces no event")
	items = getItems(t, task.ID)
	for _, it := range items {
		if it.Url == "https://testshop.bigcartel.com/product/item-c" {
			assert.False(t, it.InStock, "item-c should be OOS")
		}
	}

	// Run 4: Item C back in stock
	products[2].Status = "active"
	store.SetResponse(makeBigCartelResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 4: expected 1 restock event")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)

	// Run 5: Item B removed
	remaining := []bigcartelFixture{products[0], products[2]}
	store.SetResponse(makeBigCartelResponse(remaining...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 5: expected 1 delisted event")
	assert.Equal(t, monitor.EventDelisted, result.Events[0].Type)

	// Run 6: Item B reappears
	store.SetResponse(makeBigCartelResponse(products...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 6: expected 1 restock event (un-delist)")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)
}

func TestE2E_Squarespace(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	truncateTables(t)
	task := createTestTask(t, "squarespace", server.URL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := squarespace.NewSearchSquarespace(server.URL, client)
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	ctx := context.Background()

	items := []sqsFixture{
		{ID: "sqs-1", Title: "SQS Item 1", URLId: "sqs-item-1", InStock: true, ImageURL: "https://img.test/1.jpg"},
		{ID: "sqs-2", Title: "SQS Item 2", URLId: "sqs-item-2", InStock: true, ImageURL: "https://img.test/2.jpg"},
		{ID: "sqs-3", Title: "SQS Item 3", URLId: "sqs-item-3", InStock: true, ImageURL: "https://img.test/3.jpg"},
	}

	// Run 1: 3 new products
	store.SetResponse(makeSquarespaceResponse(items, false, 0))
	result, err := searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Events, 3, "run 1: expected 3 new_product events")
	dbItems := getItems(t, task.ID)
	require.Len(t, dbItems, 3)
	for _, it := range dbItems {
		assert.True(t, it.InStock)
		assert.False(t, it.Delisted)
	}

	// Run 2: Same — no events
	store.SetResponse(makeSquarespaceResponse(items, false, 0))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 2: expected 0 events")

	// Run 3: Item 3 goes OOS
	items[2].InStock = false
	store.SetResponse(makeSquarespaceResponse(items, false, 0))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 3: OOS transition produces no event")

	// Run 4: Item 3 back in stock
	items[2].InStock = true
	store.SetResponse(makeSquarespaceResponse(items, false, 0))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 4: expected 1 restock event")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)

	// Run 5: Item 2 removed
	remaining := []sqsFixture{items[0], items[2]}
	store.SetResponse(makeSquarespaceResponse(remaining, false, 0))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 5: expected 1 delisted event")
	assert.Equal(t, monitor.EventDelisted, result.Events[0].Type)

	// Run 6: Item 2 reappears
	store.SetResponse(makeSquarespaceResponse(items, false, 0))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 6: expected 1 restock event (un-delist)")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)
}

func TestE2E_Squarespace_Pagination(t *testing.T) {
	store := &fakeSquarespaceStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	truncateTables(t)
	task := createTestTask(t, "squarespace", server.URL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := squarespace.NewSearchSquarespace(server.URL, client)
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	ctx := context.Background()

	// Page 1: 2 items, has next page at offset 2
	page1 := []sqsFixture{
		{ID: "p1", Title: "Page 1 Item 1", URLId: "p1-item-1", InStock: true},
		{ID: "p2", Title: "Page 1 Item 2", URLId: "p1-item-2", InStock: true},
	}
	store.SetPage(0, makeSquarespaceResponse(page1, true, 2))

	// Page 2: 2 more items, no next page
	page2 := []sqsFixture{
		{ID: "p3", Title: "Page 2 Item 1", URLId: "p2-item-1", InStock: true},
		{ID: "p4", Title: "Page 2 Item 2", URLId: "p2-item-2", InStock: true},
	}
	store.SetPage(2, makeSquarespaceResponse(page2, false, 0))

	result, err := searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Events, 4, "pagination: expected 4 new_product events across 2 pages")
	items := getItems(t, task.ID)
	assert.Len(t, items, 4, "pagination: expected 4 items in DB")
}

func TestE2E_Reddit(t *testing.T) {
	store := &fakeStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	truncateTables(t)
	subredditURL := "https://www.reddit.com/r/testsubreddit"
	task := createTestTask(t, "reddit", subredditURL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := reddit.NewSearchReddit(subredditURL, client, &monitor.Opts{BaseURL: server.URL})
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	ctx := context.Background()

	posts := []redditFixture{
		{ID: "abc1", Title: "Post A", Permalink: "/r/testsubreddit/comments/abc1/post_a/", Thumbnail: "https://img.test/a.jpg"},
		{ID: "abc2", Title: "Post B", Permalink: "/r/testsubreddit/comments/abc2/post_b/", Thumbnail: "https://img.test/b.jpg"},
		{ID: "abc3", Title: "Post C", Permalink: "/r/testsubreddit/comments/abc3/post_c/", Thumbnail: "https://img.test/c.jpg"},
	}

	// Run 1: 3 new posts
	store.SetResponse(makeRedditResponse(posts...))
	result, err := searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Events, 3, "run 1: expected 3 new_product events")
	items := getItems(t, task.ID)
	require.Len(t, items, 3)
	for _, it := range items {
		assert.True(t, it.InStock, "reddit posts are always in stock")
		assert.False(t, it.Delisted)
	}

	// Run 2: Same posts — no events
	store.SetResponse(makeRedditResponse(posts...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "run 2: expected 0 events")

	// Run 3: Post B removed (delisted)
	remaining := []redditFixture{posts[0], posts[2]}
	store.SetResponse(makeRedditResponse(remaining...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 3: expected 1 delisted event")
	assert.Equal(t, monitor.EventDelisted, result.Events[0].Type)
	items = getItems(t, task.ID)
	for _, it := range items {
		if it.Url == "https://www.reddit.com/r/testsubreddit/comments/abc2/post_b/" {
			assert.True(t, it.Delisted, "post B should be delisted")
		}
	}

	// Run 4: Post B reappears — restock (un-delist)
	store.SetResponse(makeRedditResponse(posts...))
	result, err = searchTask.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Events, 1, "run 4: expected 1 restock event (un-delist)")
	assert.Equal(t, monitor.EventRestock, result.Events[0].Type)
	items = getItems(t, task.ID)
	for _, it := range items {
		assert.False(t, it.Delisted)
	}
}

func TestSchedulerDispatch(t *testing.T) {
	truncateTables(t)

	store := &fakeStore{}
	server := httptest.NewServer(store)
	defer server.Close()

	task := createTestTask(t, "shopify", server.URL)
	queries := database.New(testDB)

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := shopify.NewSearchShopify(server.URL, client)
	searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

	notifier := &mockNotifier{}

	// --- Run 1: seed items, first run is always suppressed ---
	store.SetResponse(makeShopifyResponse(
		shopifyFixture{Handle: "p1", Title: "Product 1", Available: true},
		shopifyFixture{Handle: "p2", Title: "Product 2", Available: false},
	))
	runSchedulerOnce(t, searchTask, queries, notifier)
	assert.Empty(t, notifier.calls, "first run should not notify")

	// --- Create subscriptions ---
	// "all" on channel-a, "new" on channel-b
	_, err = testDB.Exec(
		`INSERT INTO task_subscriptions (task_id, channel_id, mode) VALUES ($1, 'channel-a', 'all')`, task.ID)
	require.NoError(t, err)
	_, err = testDB.Exec(
		`INSERT INTO task_subscriptions (task_id, channel_id, mode) VALUES ($1, 'channel-b', 'new')`, task.ID)
	require.NoError(t, err)

	// "restock" on channel-c for product p2
	items := getItems(t, task.ID)
	var p2ID int32
	for _, it := range items {
		if strings.Contains(it.Url, "p2") {
			p2ID = it.ID
		}
	}
	require.NotZero(t, p2ID, "should find p2 in DB")
	_, err = testDB.Exec(
		`INSERT INTO task_subscriptions (task_id, channel_id, mode, item_id) VALUES ($1, 'channel-c', 'restock', $2)`,
		task.ID, p2ID)
	require.NoError(t, err)

	// --- Run 2: p2 restocks (OOS → IS) ---
	store.SetResponse(makeShopifyResponse(
		shopifyFixture{Handle: "p1", Title: "Product 1", Available: true},
		shopifyFixture{Handle: "p2", Title: "Product 2", Available: true},
	))
	notifier.reset()
	runSchedulerOnce(t, searchTask, queries, notifier)

	// Restock event should reach channel-a (all) and channel-c (restock for p2),
	// but NOT channel-b (new mode ignores restock events).
	require.Len(t, notifier.calls, 2, "restock should notify 2 channels")
	channels := []string{notifier.calls[0].channelID, notifier.calls[1].channelID}
	assert.Contains(t, channels, "channel-a")
	assert.Contains(t, channels, "channel-c")

	// --- Run 3: new product p3 appears ---
	store.SetResponse(makeShopifyResponse(
		shopifyFixture{Handle: "p1", Title: "Product 1", Available: true},
		shopifyFixture{Handle: "p2", Title: "Product 2", Available: true},
		shopifyFixture{Handle: "p3", Title: "Product 3", Available: true},
	))
	notifier.reset()
	runSchedulerOnce(t, searchTask, queries, notifier)

	// new_product event should reach channel-a (all) and channel-b (new),
	// but NOT channel-c (restock mode ignores new_product events).
	require.Len(t, notifier.calls, 2, "new_product should notify 2 channels")
	channels = []string{notifier.calls[0].channelID, notifier.calls[1].channelID}
	assert.Contains(t, channels, "channel-a")
	assert.Contains(t, channels, "channel-b")

	// --- Run 4: p1 delisted ---
	store.SetResponse(makeShopifyResponse(
		shopifyFixture{Handle: "p2", Title: "Product 2", Available: true},
		shopifyFixture{Handle: "p3", Title: "Product 3", Available: true},
	))
	notifier.reset()
	runSchedulerOnce(t, searchTask, queries, notifier)

	// Delisted event for p1 should reach channel-a (all) only.
	// channel-b (new) ignores delisted. channel-c (restock) is for p2, not p1.
	require.Len(t, notifier.calls, 1, "delist of p1 should notify 1 channel")
	assert.Equal(t, "channel-a", notifier.calls[0].channelID)
}
