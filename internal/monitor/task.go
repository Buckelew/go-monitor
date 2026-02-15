package task

import (
	"context"
	"time"
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

type TaskResult struct {
	Success     bool
	NewProducts []Product
	Error       error
}

type Task interface {
	ID() string
	Platform() Platform
	Type() TaskType
	Run(ctx context.Context) (*TaskResult, error)
	Delay() time.Duration
	IsEnabled() bool
}
