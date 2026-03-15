package taskbuild

import (
	"context"
	"fmt"
	"log"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/bigcartel"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/reddit"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/shopify"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/squarespace"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/target"
	"github.com/buckelew/go-monitor/internal/solver"
)

// BuildTask creates a Task from a database.Task row by loading proxies,
// creating an HTTP client, and instantiating the platform-specific scraper.
func BuildTask(ctx context.Context, queries *database.Queries, dbTask database.Task, solverClient *solver.Client, proxyRegistry *httpclient.ProxyRegistry) (monitor.Task, error) {
	var rotator *httpclient.ProxyRotator
	if dbTask.ProxyListID.Valid {
		dbProxies, err := queries.GetProxiesByListID(ctx, dbTask.ProxyListID.Int32)
		if err != nil {
			return nil, fmt.Errorf("load proxies for task %d: %w", dbTask.ID, err)
		}
		if len(dbProxies) > 0 {
			var proxyURLs []string
			for _, p := range dbProxies {
				if p.Username.Valid && p.Username.String != "" {
					proxyURLs = append(proxyURLs, fmt.Sprintf("http://%s:%s@%s:%s", p.Username.String, p.Password.String, p.Host, p.Port))
				} else {
					proxyURLs = append(proxyURLs, fmt.Sprintf("http://%s:%s", p.Host, p.Port))
				}
			}
			rotator = httpclient.NewProxyRotator(proxyURLs)
		}
	}

	client, err := httpclient.New(rotator)
	if err != nil {
		return nil, fmt.Errorf("create http client for task %d: %w", dbTask.ID, err)
	}

	// Target ATC tasks have a different task type and don't use the Scraper interface.
	if dbTask.Platform == string(monitor.Target) && dbTask.TaskType == string(monitor.ATC) {
		tcin, err := monitor.ExtractTargetTCIN(dbTask.Url)
		if err != nil {
			return nil, fmt.Errorf("extract TCIN for task %d: %w", dbTask.ID, err)
		}
		return target.NewATCTask(*queries, client, solverClient, tcin, dbTask), nil
	}

	var scraper monitor.Scraper
	switch dbTask.Platform {
	case string(monitor.Shopify):
		if dbTask.TaskType == string(monitor.Search) {
			scraper = shopify.NewSearchShopify(dbTask.Url, client)
		}
	case string(monitor.Bigcartel):
		if dbTask.TaskType == string(monitor.Search) {
			scraper = bigcartel.NewSearchBigCartel(dbTask.Url, client)
		}
	case string(monitor.Squarespace):
		if dbTask.TaskType == string(monitor.Search) {
			scraper = squarespace.NewSearchSquarespace(dbTask.Url, client)
		}
	case string(monitor.Reddit):
		if dbTask.TaskType == string(monitor.Search) {
			scraper = reddit.NewSearchReddit(dbTask.Url, client)
		}
	}

	if scraper == nil {
		return nil, fmt.Errorf("unsupported platform/type: %s/%s", dbTask.Platform, dbTask.TaskType)
	}

	return monitor.NewSearchTask(*queries, scraper, dbTask, proxyRegistry), nil
}

// BuildInitialTasks builds Task values from a slice of database tasks,
// skipping disabled tasks and logging errors.
func BuildInitialTasks(ctx context.Context, queries *database.Queries, dbTasks []database.Task, solverClient *solver.Client, proxyRegistry *httpclient.ProxyRegistry) []monitor.Task {
	var tasks []monitor.Task
	for _, dbTask := range dbTasks {
		if !dbTask.Enabled {
			continue
		}
		task, err := BuildTask(ctx, queries, dbTask, solverClient, proxyRegistry)
		if err != nil {
			log.Printf("skip task %d: %v", dbTask.ID, err)
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}
