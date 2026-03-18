package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/t0nyandre/gltchbot/internal/bot"
	"github.com/t0nyandre/gltchbot/internal/config"
	"github.com/t0nyandre/gltchbot/internal/db"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

func main() {
	// Initialize structured logging
	logging.Init(logging.DefaultConfig())

	cfg, err := config.Load()
	if err != nil {
		logging.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	// Connect to PostgreSQL and run migrations
	pool, err := db.New(ctx, cfg)
	if err != nil {
		logging.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create and start the Discord bot
	client, err := bot.New(cfg, pool)
	if err != nil {
		logging.Fatalf("failed to create bot client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		logging.Fatalf("failed to start bot: %v", err)
	}
	defer client.Close()

	logging.Info("bot is running. press Ctrl+C to exit.")

	// Block until interrupted
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logging.Info("shutting down bot...")
}
