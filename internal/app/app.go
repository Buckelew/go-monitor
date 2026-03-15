package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/buckelew/go-monitor/config"
	"github.com/buckelew/go-monitor/internal/api"
	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/discord"
	"github.com/buckelew/go-monitor/internal/hub"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/taskbuild"
	"github.com/buckelew/go-monitor/internal/web"
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

	// Web dashboard
	webServer := web.NewServer(queries, db)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Web.Port)
		log.Printf("web dashboard: http://localhost%s", addr)
		if err := http.ListenAndServe(addr, webServer); err != nil && err != http.ErrServerClosed {
			log.Printf("web server error: %v", err)
		}
	}()

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

	// Event hub
	eventHub := hub.New()

	// Discord hub subscriber — bridges hub events to Discord notifications
	discordSub := discord.NewHubSubscriber(eventHub, notifier, queries)
	go discordSub.Run(ctx)

	// Build initial tasks
	dbTasks, err := queries.GetTasks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	initialTasks := taskbuild.BuildInitialTasks(ctx, queries, dbTasks)

	// Scheduler
	scheduler := monitor.NewScheduler(queries, eventHub)

	// API server
	apiServer := api.NewServer(queries, db, eventHub, scheduler, bot, cfg, func(ctx context.Context, q *database.Queries, t database.Task) (monitor.Task, error) {
		return taskbuild.BuildTask(ctx, q, t)
	})
	go func() {
		addr := fmt.Sprintf(":%d", cfg.API.Port)
		log.Printf("api server: http://localhost%s", addr)
		if err := http.ListenAndServe(addr, apiServer); err != nil && err != http.ErrServerClosed {
			log.Printf("api server error: %v", err)
		}
	}()

	if len(initialTasks) == 0 {
		log.Println("no enabled tasks found, servers still running")
		<-ctx.Done()
	} else {
		log.Printf("starting scheduler with %d task(s)", len(initialTasks))
		scheduler.Start(ctx, initialTasks)
	}
	log.Println("shutdown complete")
}
