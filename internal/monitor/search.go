package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
)

type Scraper interface {
	FetchProducts(ctx context.Context) (*FetchResult, error)
	Platform() Platform
}

// ScraperWithClient is implemented by scrapers that expose their HTTP client
// for proxy hot-swapping.
type ScraperWithClient interface {
	Scraper
	Client() *httpclient.Client
}

type SearchTask struct {
	id           int32
	url          string
	scraper      Scraper
	isEnabled    bool
	queries      *database.Queries
	proxyListID  sql.NullInt32
	registry     *httpclient.ProxyRegistry
	proxyVersion int64
	passwordPage bool
	seenSuccess  bool // true after first successful (non-password) fetch
}

func (s *SearchTask) ID() int32          { return s.id }
func (s *SearchTask) Platform() Platform { return s.scraper.Platform() }
func (s *SearchTask) Type() TaskType     { return Search }
func (s *SearchTask) IsEnabled() bool    { return s.isEnabled }

func (s *SearchTask) Run(ctx context.Context) (*TaskResult, error) {
	s.maybeRefreshProxies(ctx)

	fetched, err := s.scraper.FetchProducts(ctx)
	if errors.Is(err, ErrPasswordPage) {
		var events []ItemEvent
		if !s.passwordPage {
			s.passwordPage = true
			log.Printf("[task %d] password page up: %s", s.id, s.url)
			if s.seenSuccess {
				events = append(events, ItemEvent{
					Type: EventPasswordUp,
					Item: Item{URL: s.url, Title: "Password page up"},
				})
			}
		}
		return &TaskResult{Success: true, Events: events}, nil
	}
	if err != nil {
		return nil, err
	}

	s.seenSuccess = true
	meta := fetched.Meta
	var events []ItemEvent

	// Password came down
	if s.passwordPage {
		s.passwordPage = false
		log.Printf("[task %d] password page down: %s", s.id, s.url)
		events = append(events, ItemEvent{
			Type: EventPasswordDown,
			Item: Item{URL: s.url, Title: "Password page down"},
		})
	}
	// Pre-fetch all known items for this task in one query (no Data column).
	// This replaces N per-item GetItemByTaskAndURL calls with 1 bulk query,
	// eliminating ~1000 DB round trips per cycle for large stores.
	summaries, err := s.queries.GetItemSummariesByTask(ctx, s.id)
	if err != nil {
		return nil, fmt.Errorf("failed to pre-fetch item summaries: %w", err)
	}
	knownItems := make(map[string]database.GetItemSummariesByTaskRow, len(summaries))
	for _, row := range summaries {
		knownItems[row.Url] = row
	}

	scrapedURLs := make(map[string]bool, len(fetched.Items))
	for i, item := range fetched.Items {
		scrapedURLs[item.URL] = true

		existing, found := knownItems[item.URL]

		if !found {
			// New product — insert it
			created, err := s.queries.UpsertItem(ctx, database.UpsertItemParams{
				TaskID:   s.id,
				Url:      item.URL,
				Platform: string(s.Platform()),
				Data:     item.Data,
				InStock:  item.InStock,
			})
			if err != nil {
				log.Printf("[task %d] failed to insert item %s: %v", s.id, item.URL, err)
				fetched.Items[i].Data = nil
				continue
			}

			events = append(events, ItemEvent{Type: EventNewProduct, Item: item, ItemID: created.ID})
			s.insertEvent(ctx, created.ID, json.RawMessage("{}"), item.Data)
			fetched.Items[i].Data = nil
			continue
		}

		// Un-delist if it was previously delisted (product reappeared)
		if existing.Delisted {
			if err := s.queries.UndelistItem(ctx, existing.ID); err != nil {
				log.Printf("[task %d] failed to un-delist item %s: %v", s.id, item.URL, err)
			}
			prevData := s.getItemData(ctx, existing.ID)
			events = append(events, ItemEvent{Type: EventRestock, Item: item, ItemID: existing.ID})
			s.insertEvent(ctx, existing.ID, prevData, item.Data)
		}

		// Check for restock (OOS → IS)
		if !existing.Delisted && !existing.InStock && item.InStock {
			prevData := s.getItemData(ctx, existing.ID)
			events = append(events, ItemEvent{Type: EventRestock, Item: item, ItemID: existing.ID})
			s.insertEvent(ctx, existing.ID, prevData, item.Data)
		}

		// Update data unconditionally — let the DB skip if unchanged via
		// the WHERE clause. Avoids fetching existing Data into Go memory
		// just to compare (~15KB per item × 1000+ items = major allocation).
		s.queries.UpdateItemDataIfChanged(ctx, database.UpdateItemDataIfChangedParams{
			ID:   existing.ID,
			Data: item.Data,
		})

		// Update in_stock if changed
		if existing.InStock != item.InStock {
			if err := s.queries.UpdateItemInStock(ctx, database.UpdateItemInStockParams{
				ID:      existing.ID,
				InStock: item.InStock,
			}); err != nil {
				log.Printf("[task %d] failed to update in_stock for %s: %v", s.id, item.URL, err)
			}
		}

		fetched.Items[i].Data = nil
	}

	// Delisting detection: any known active item not in the scraped set
	for url, row := range knownItems {
		if row.Delisted || scrapedURLs[url] {
			continue
		}
		if err := s.queries.MarkItemDelisted(ctx, row.ID); err != nil {
			log.Printf("[task %d] failed to mark item %s delisted: %v", s.id, url, err)
			continue
		}
		prevData := s.getItemData(ctx, row.ID)
		title, imageURL := extractItemInfo(prevData)
		events = append(events, ItemEvent{
			Type:   EventDelisted,
			Item:   Item{URL: url, Title: title, ImageURL: imageURL, Data: prevData},
			ItemID: row.ID,
		})
		s.insertEvent(ctx, row.ID, prevData, json.RawMessage(`{"delisted": true}`))
	}

	return &TaskResult{Success: true, Events: events, Meta: &meta}, nil
}

func (s *SearchTask) getItemData(ctx context.Context, itemID int32) json.RawMessage {
	data, err := s.queries.GetItemDataByID(ctx, itemID)
	if err != nil {
		log.Printf("[task %d] failed to fetch item data for %d: %v", s.id, itemID, err)
		return json.RawMessage("{}")
	}
	return data
}

func (s *SearchTask) insertEvent(ctx context.Context, itemID int32, prevState, newState json.RawMessage) {
	_, err := s.queries.InsertItemEvent(ctx, database.InsertItemEventParams{
		ItemID:        itemID,
		PreviousState: prevState,
		NewState:      newState,
	})
	if err != nil {
		log.Printf("[task %d] failed to insert item event: %v", s.id, err)
	}
}

func (s *SearchTask) maybeRefreshProxies(ctx context.Context) {
	if s.registry == nil || !s.proxyListID.Valid {
		return
	}

	currentVersion := s.registry.Version(s.proxyListID.Int32)
	if currentVersion == s.proxyVersion {
		return
	}

	swc, ok := s.scraper.(ScraperWithClient)
	if !ok {
		return
	}

	dbProxies, err := s.queries.GetProxiesByListID(ctx, s.proxyListID.Int32)
	if err != nil {
		log.Printf("[task %d] failed to reload proxies: %v", s.id, err)
		return
	}

	var proxyURLs []string
	for _, p := range dbProxies {
		if p.Username.Valid && p.Username.String != "" {
			proxyURLs = append(proxyURLs, fmt.Sprintf("http://%s:%s@%s:%s", p.Username.String, p.Password.String, p.Host, p.Port))
		} else {
			proxyURLs = append(proxyURLs, fmt.Sprintf("http://%s:%s", p.Host, p.Port))
		}
	}

	if len(proxyURLs) > 0 {
		rotator := httpclient.NewProxyRotator(proxyURLs)
		rotator = s.registry.GetOrSetRotator(s.proxyListID.Int32, rotator)
		swc.Client().SetRotator(rotator)
	}

	s.proxyVersion = currentVersion
	log.Printf("[task %d] proxies refreshed (version %d)", s.id, currentVersion)
}

// extractItemInfo extracts title and image URL from stored item data JSON.
// Handles all platform formats: Shopify (title, images[].src), BigCartel
// (name, images[].secure_url), Squarespace (title, assetUrl), Reddit (title, thumbnail).
func extractItemInfo(data json.RawMessage) (title, imageURL string) {
	if len(data) == 0 {
		return "", ""
	}
	var d struct {
		Title     string `json:"title"`
		Name      string `json:"name"`
		AssetURL  string `json:"assetUrl"`
		Thumbnail string `json:"thumbnail"`
		Images    []struct {
			Src       string `json:"src"`
			SecureURL string `json:"secure_url"`
		} `json:"images"`
	}
	if json.Unmarshal(data, &d) != nil {
		return "", ""
	}

	title = d.Title
	if title == "" {
		title = d.Name
	}

	if len(d.Images) > 0 {
		if d.Images[0].Src != "" {
			imageURL = d.Images[0].Src
		} else if d.Images[0].SecureURL != "" {
			imageURL = d.Images[0].SecureURL
		}
	}
	if imageURL == "" && d.AssetURL != "" {
		imageURL = d.AssetURL
	}
	if imageURL == "" && d.Thumbnail != "" &&
		d.Thumbnail != "self" && d.Thumbnail != "default" && d.Thumbnail != "nsfw" {
		imageURL = d.Thumbnail
	}

	return title, imageURL
}

func NewSearchTask(queries database.Queries, scraper Scraper, task database.Task, registry *httpclient.ProxyRegistry) *SearchTask {
	return &SearchTask{
		id:          task.ID,
		url:         task.Url,
		scraper:     scraper,
		isEnabled:   task.Enabled,
		queries:     &queries,
		proxyListID: task.ProxyListID,
		registry:    registry,
	}
}
