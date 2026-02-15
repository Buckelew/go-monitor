package main

import (
	"log"

	"github.com/buckelew/go-monitor/config"
	"github.com/buckelew/go-monitor/internal/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	// Run
	app.Run(cfg)
}
