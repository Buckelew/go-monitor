package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Platform string

const (
	Bigcartel   Platform = "bigcartel"
	Reddit      Platform = "reddit"
	Shopify     Platform = "shopify"
	Squarespace Platform = "squarespace"
	Target      Platform = "target"
)

type TaskType string

const (
	ATC     TaskType = "atc"
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
	EventNewProduct   EventType = "new_product"
	EventRestock      EventType = "restock"
	EventDelisted     EventType = "delisted"
	EventPasswordUp   EventType = "password_up"
	EventPasswordDown EventType = "password_down"
)

// ErrPasswordPage is returned by scrapers when the store has a password page
// (Shopify 401, BigCartel 403). The scheduler treats this as a non-error.
var ErrPasswordPage = errors.New("password page")

type ItemEvent struct {
	Type   EventType
	Item   Item
	ItemID int32 // DB item ID, needed for product-level subscription matching
}

type FetchMeta struct {
	StatusCode  int
	CacheStatus string
	Duration    time.Duration
	BodySize    int
	Proxy       string
}

type FetchResult struct {
	Items []Item
	Meta  FetchMeta
}

type TaskResult struct {
	Success bool
	Events  []ItemEvent
	Meta    *FetchMeta
}

type Notifier interface {
	Notify(channelID string, event ItemEvent) error
}

type Task interface {
	ID() int32
	Platform() Platform
	Type() TaskType
	Run(ctx context.Context) (*TaskResult, error)
	IsEnabled() bool
}
