package monitor

import (
	"context"
	"encoding/json"
)

type Platform string

const (
	Bigcartel   Platform = "bigcartel"
	Reddit      Platform = "reddit"
	Shopify     Platform = "shopify"
	Squarespace Platform = "squarespace"
)

type TaskType string

const (
	Restock TaskType = "restock"
	Search  TaskType = "search"
)

// Opts allows overriding scraper behavior (e.g. base URL for tests).
type Opts struct {
	BaseURL string // override base URL for HTTP requests
}

type Item struct {
	URL      string
	Title    string
	InStock  bool
	ImageURL string
	Data     json.RawMessage
}

type EventType string

const (
	EventNewProduct EventType = "new_product"
	EventRestock    EventType = "restock"
	EventDelisted   EventType = "delisted"
)

type ItemEvent struct {
	Type   EventType
	Item   Item
	ItemID int32 // DB item ID, needed for product-level subscription matching
}

type TaskResult struct {
	Success bool
	Events  []ItemEvent
}

type Notifier interface {
	Notify(channelID string, event ItemEvent) error
}

type Task interface {
	ID() int32
	Platform() Platform
	Type() TaskType
	Run(ctx context.Context) (*TaskResult, error)
	Delay() int32
	IsEnabled() bool
}
