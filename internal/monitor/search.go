package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
)

type Scraper interface {
	FetchProducts(ctx context.Context) ([]Item, error)
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
	scraper      Scraper
	delay        int32
	isEnabled    bool
	queries      *database.Queries
	proxyListID  sql.NullInt32
	registry     *httpclient.ProxyRegistry
	proxyVersion int64
}

func (s *SearchTask) ID() int32          { return s.id }
func (s *SearchTask) Platform() Platform { return s.scraper.Platform() }
func (s *SearchTask) Type() TaskType     { return Search }
func (s *SearchTask) Delay() int32       { return s.delay }
func (s *SearchTask) IsEnabled() bool    { return s.isEnabled }

func (s *SearchTask) Run(ctx context.Context) (*TaskResult, error) {
	s.maybeRefreshProxies(ctx)

	items, err := s.scraper.FetchProducts(ctx)
	if err != nil {
		return nil, err
	}

	var events []ItemEvent
	for _, item := range items {
		existing, err := s.queries.GetItemByTaskAndURL(ctx, database.GetItemByTaskAndURLParams{
			TaskID: s.id,
			Url:    item.URL,
		})

		if err == sql.ErrNoRows {
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
				continue
			}

			events = append(events, ItemEvent{Type: EventNewProduct, Item: item, ItemID: created.ID})

			// Log the event in DB
			s.insertEvent(ctx, created.ID, json.RawMessage("{}"), item.Data)
			continue
		}

		if err != nil {
			log.Printf("[task %d] failed to look up item %s: %v", s.id, item.URL, err)
			continue
		}

		// Un-delist if it was previously delisted (product reappeared)
		if existing.Delisted {
			if err := s.queries.UndelistItem(ctx, existing.ID); err != nil {
				log.Printf("[task %d] failed to un-delist item %s: %v", s.id, item.URL, err)
			}
			events = append(events, ItemEvent{Type: EventRestock, Item: item, ItemID: existing.ID})
			s.insertEvent(ctx, existing.ID, existing.Data, item.Data)
		}

		// Check for restock (OOS → IS)
		if !existing.Delisted && !existing.InStock && item.InStock {
			events = append(events, ItemEvent{Type: EventRestock, Item: item, ItemID: existing.ID})
			s.insertEvent(ctx, existing.ID, existing.Data, item.Data)
		}

		// Update data if changed
		if string(existing.Data) != string(item.Data) {
			if err := s.queries.UpdateItemData(ctx, database.UpdateItemDataParams{
				ID:   existing.ID,
				Data: item.Data,
			}); err != nil {
				log.Printf("[task %d] failed to update item data for %s: %v", s.id, item.URL, err)
			}
		}

		// Update in_stock if changed
		if existing.InStock != item.InStock {
			if err := s.queries.UpdateItemInStock(ctx, database.UpdateItemInStockParams{
				ID:      existing.ID,
				InStock: item.InStock,
			}); err != nil {
				log.Printf("[task %d] failed to update in_stock for %s: %v", s.id, item.URL, err)
			}
		}
	}

	// Delisting detection: any active DB item not in the scraped set is delisted
	scrapedURLs := make(map[string]bool, len(items))
	for _, item := range items {
		scrapedURLs[item.URL] = true
	}

	activeItems, err := s.queries.GetActiveItemsByTask(ctx, s.id)
	if err != nil {
		log.Printf("[task %d] failed to get active items for delisting check: %v", s.id, err)
	} else {
		for _, dbItem := range activeItems {
			if !scrapedURLs[dbItem.Url] {
				if err := s.queries.MarkItemDelisted(ctx, dbItem.ID); err != nil {
					log.Printf("[task %d] failed to mark item %s delisted: %v", s.id, dbItem.Url, err)
					continue
				}
				events = append(events, ItemEvent{
					Type:   EventDelisted,
					Item:   Item{URL: dbItem.Url, Data: dbItem.Data},
					ItemID: dbItem.ID,
				})
				s.insertEvent(ctx, dbItem.ID, dbItem.Data, json.RawMessage(`{"delisted": true}`))
			}
		}
	}

	return &TaskResult{Success: true, Events: events}, nil
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
		swc.Client().SetRotator(httpclient.NewProxyRotator(proxyURLs))
	}

	s.proxyVersion = currentVersion
	log.Printf("[task %d] proxies refreshed (version %d)", s.id, currentVersion)
}

func NewSearchTask(queries database.Queries, scraper Scraper, task database.Task, registry *httpclient.ProxyRegistry) *SearchTask {
	return &SearchTask{
		id:          task.ID,
		scraper:     scraper,
		delay:       task.Delay,
		isEnabled:   task.Enabled,
		queries:     &queries,
		proxyListID: task.ProxyListID,
		registry:    registry,
	}
}
