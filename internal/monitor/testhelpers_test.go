package monitor_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/buckelew/go-monitor/internal/database"
)

var testDB *sql.DB
var testQueries *database.Queries

func TestMain(m *testing.M) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		log.Println("POSTGRES_TEST_URL not set, skipping E2E tests")
		os.Exit(0)
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to test db: %v", err)
	}
	defer testDB.Close()

	if err := testDB.Ping(); err != nil {
		log.Fatalf("failed to ping test db: %v", err)
	}

	// Run migrations
	driver, err := postgres.WithInstance(testDB, &postgres.Config{})
	if err != nil {
		log.Fatalf("failed to create migrate driver: %v", err)
	}

	mig, err := migrate.NewWithDatabaseInstance("file://../../migrations", "postgres", driver)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}

	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to run migrations: %v", err)
	}

	testQueries = database.New(testDB)

	os.Exit(m.Run())
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE items, item_events, task_runs, tasks RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

func createTestTask(t *testing.T, platform, url string) database.Task {
	t.Helper()
	var task database.Task
	err := testDB.QueryRow(
		`INSERT INTO tasks (platform, task_type, url, delay, enabled, channel_id)
		 VALUES ($1, 'search', $2, 5000, true, 'test-channel')
		 RETURNING id, platform, task_type, url, proxy_list_id, delay, enabled, created_at, updated_at, channel_id`,
		platform, url,
	).Scan(
		&task.ID, &task.Platform, &task.TaskType, &task.Url,
		&task.ProxyListID, &task.Delay, &task.Enabled,
		&task.CreatedAt, &task.UpdatedAt, &task.ChannelID,
	)
	if err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	return task
}

// fakeStore serves canned JSON responses for httptest.Server.
type fakeStore struct {
	mu       sync.Mutex
	response []byte
}

func (f *fakeStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	data := f.response
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (f *fakeStore) SetResponse(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.response = data
}

// fakeSquarespaceStore handles pagination via offset query param.
type fakeSquarespaceStore struct {
	mu    sync.Mutex
	pages map[int][]byte // offset -> response body
}

func (f *fakeSquarespaceStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}

	f.mu.Lock()
	data := f.pages[offset]
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (f *fakeSquarespaceStore) SetPage(offset int, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pages == nil {
		f.pages = make(map[int][]byte)
	}
	f.pages[offset] = data
}

// --- JSON fixture builders ---

type shopifyFixture struct {
	Handle    string
	Title     string
	Available bool
	ImageSrc  string
}

func makeShopifyResponse(products ...shopifyFixture) []byte {
	type variant struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Available bool   `json:"available"`
		Price     string `json:"price"`
	}
	type image struct {
		Src string `json:"src"`
	}
	type product struct {
		ID       int64     `json:"id"`
		Title    string    `json:"title"`
		Handle   string    `json:"handle"`
		Variants []variant `json:"variants"`
		Images   []image   `json:"images"`
	}

	var rawProducts []json.RawMessage
	for i, p := range products {
		prod := product{
			ID:     int64(i + 1),
			Title:  p.Title,
			Handle: p.Handle,
			Variants: []variant{
				{ID: int64(i*10 + 1), Title: "Default", Available: p.Available, Price: "10.00"},
			},
		}
		if p.ImageSrc != "" {
			prod.Images = []image{{Src: p.ImageSrc}}
		}
		raw, _ := json.Marshal(prod)
		rawProducts = append(rawProducts, raw)
	}

	resp := struct {
		Products []json.RawMessage `json:"products"`
	}{Products: rawProducts}
	data, _ := json.Marshal(resp)
	return data
}

type bigcartelFixture struct {
	Name      string
	Permalink string
	Status    string // "active" or "sold_out"
	ImageURL  string
}

func makeBigCartelResponse(products ...bigcartelFixture) []byte {
	type option struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		SoldOut bool   `json:"sold_out"`
	}
	type image struct {
		URL       string `json:"url"`
		SecureURL string `json:"secure_url"`
	}
	type product struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		Permalink string   `json:"permalink"`
		Status    string   `json:"status"`
		Options   []option `json:"options"`
		Images    []image  `json:"images"`
	}

	var prods []product
	for i, p := range products {
		prod := product{
			ID:        i + 1,
			Name:      p.Name,
			Permalink: p.Permalink,
			Status:    p.Status,
			Options:   []option{{ID: i*10 + 1, Name: "Default", SoldOut: p.Status == "sold_out"}},
		}
		if p.ImageURL != "" {
			prod.Images = []image{{URL: p.ImageURL, SecureURL: p.ImageURL}}
		}
		prods = append(prods, prod)
	}

	data, _ := json.Marshal(prods)
	return data
}

type sqsFixture struct {
	ID    string
	Title string
	URLId string
	// If nil, structuredContent is omitted (no variants = OOS)
	InStock  bool
	ImageURL string
}

func makeSquarespaceResponse(items []sqsFixture, hasNext bool, nextOffset int) []byte {
	type variant struct {
		ID         string `json:"id"`
		QtyInStock int    `json:"qtyInStock"`
		Unlimited  bool   `json:"unlimited"`
	}
	type structuredContent struct {
		Variants []variant `json:"variants"`
	}
	type item struct {
		ID                string             `json:"id"`
		Title             string             `json:"title"`
		URLId             string             `json:"urlId"`
		FullURL           string             `json:"fullUrl"`
		AssetURL          string             `json:"assetUrl"`
		StructuredContent *structuredContent `json:"structuredContent,omitempty"`
	}
	type pagination struct {
		NextPage       bool `json:"nextPage"`
		NextPageOffset int  `json:"nextPageOffset"`
	}

	var rawItems []json.RawMessage
	for i, si := range items {
		it := item{
			ID:       si.ID,
			Title:    si.Title,
			URLId:    si.URLId,
			AssetURL: si.ImageURL,
		}
		if si.InStock {
			it.StructuredContent = &structuredContent{
				Variants: []variant{{ID: fmt.Sprintf("v%d", i+1), Unlimited: true}},
			}
		} else {
			it.StructuredContent = &structuredContent{
				Variants: []variant{{ID: fmt.Sprintf("v%d", i+1), QtyInStock: 0}},
			}
		}
		raw, _ := json.Marshal(it)
		rawItems = append(rawItems, raw)
	}

	resp := struct {
		Items      []json.RawMessage `json:"items"`
		Pagination *pagination       `json:"pagination"`
	}{
		Items: rawItems,
		Pagination: &pagination{
			NextPage:       hasNext,
			NextPageOffset: nextOffset,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

type redditFixture struct {
	ID        string
	Title     string
	Permalink string
	Thumbnail string
}

func makeRedditResponse(posts ...redditFixture) []byte {
	type post struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Permalink string `json:"permalink"`
		Thumbnail string `json:"thumbnail"`
	}
	type child struct {
		Data post `json:"data"`
	}
	type data struct {
		Children []child `json:"children"`
	}
	type response struct {
		Data data `json:"data"`
	}

	var children []child
	for _, p := range posts {
		children = append(children, child{Data: post{
			ID:        p.ID,
			Title:     p.Title,
			Permalink: p.Permalink,
			Thumbnail: p.Thumbnail,
		}})
	}

	resp := response{Data: data{Children: children}}
	d, _ := json.Marshal(resp)
	return d
}

// getItems returns all items for a given task, ordered by ID.
func getItems(t *testing.T, taskID int32) []database.Item {
	t.Helper()
	rows, err := testDB.Query(
		"SELECT id, task_id, url, platform, created_at, data, delisted, in_stock FROM items WHERE task_id = $1 ORDER BY id", taskID,
	)
	if err != nil {
		t.Fatalf("failed to query items: %v", err)
	}
	defer rows.Close()

	var items []database.Item
	for rows.Next() {
		var i database.Item
		if err := rows.Scan(&i.ID, &i.TaskID, &i.Url, &i.Platform, &i.CreatedAt, &i.Data, &i.Delisted, &i.InStock); err != nil {
			t.Fatalf("failed to scan item: %v", err)
		}
		items = append(items, i)
	}
	return items
}

func getItemEvents(t *testing.T, itemID int32) []database.ItemEvent {
	t.Helper()
	rows, err := testDB.Query(
		"SELECT id, item_id, created_at, previous_state, new_state, webhook_sent FROM item_events WHERE item_id = $1 ORDER BY id", itemID,
	)
	if err != nil {
		t.Fatalf("failed to query item_events: %v", err)
	}
	defer rows.Close()

	var events []database.ItemEvent
	for rows.Next() {
		var e database.ItemEvent
		if err := rows.Scan(&e.ID, &e.ItemID, &e.CreatedAt, &e.PreviousState, &e.NewState, &e.WebhookSent); err != nil {
			t.Fatalf("failed to scan item event: %v", err)
		}
		events = append(events, e)
	}
	return events
}
