package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/buckelew/go-monitor/config"
	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/discord"
	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/shopify"
)

func Run(cfg *config.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for interrupt signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	queries, db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Discord bot
	bot, err := discord.NewBot(cfg.Discord.Token)
	if err != nil {
		log.Fatal(err)
	}
	if err := bot.Open(); err != nil {
		log.Fatal(err)
	}
	defer bot.Close()
	log.Println("discord bot connected")

	notifier := discord.NewNotifier(bot)

	dbTasks, err := queries.GetTasks(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var tasks []monitor.Task
	for _, dbTask := range dbTasks {
		if !dbTask.Enabled {
			continue
		}

		// Build proxy rotator and HTTP client for this task
		var rotator *httpclient.ProxyRotator
		if dbTask.ProxyListID.Valid {
			dbProxies, err := queries.GetProxiesByListID(ctx, dbTask.ProxyListID.Int32)
			if err != nil {
				log.Printf("failed to load proxies for task %d: %v", dbTask.ID, err)
			} else if len(dbProxies) > 0 {
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
			log.Printf("failed to create http client for task %d: %v", dbTask.ID, err)
			continue
		}

		var scraper monitor.Scraper

		switch dbTask.Platform {
		case string(monitor.Shopify):
			switch dbTask.TaskType {
			case string(monitor.Search):
				scraper = shopify.NewSearchShopify(dbTask.Url, client)
			}
		}

		if scraper == nil {
			log.Printf("unsupported platform/type: %s/%s", dbTask.Platform, dbTask.TaskType)
			continue
		}

		task := monitor.NewSearchTask(*queries, scraper, dbTask)
		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		log.Println("no enabled tasks found")
		return
	}

	log.Printf("starting scheduler with %d task(s)", len(tasks))
	scheduler := monitor.NewScheduler(tasks, queries, notifier)
	scheduler.Start(ctx)
	log.Println("shutdown complete")
}
