package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/t0nyandre/gltchbot/internal/bot"
	"github.com/t0nyandre/gltchbot/internal/config"
	"github.com/t0nyandre/gltchbot/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	// Connect to PostgreSQL and run migrations
	pool, err := db.New(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create and start the Discord bot
	client, err := bot.New(cfg, pool)
	if err != nil {
		log.Fatalf("failed to create bot client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		log.Fatalf("failed to start bot: %v", err)
	}
	defer client.Close()

	log.Println("bot is running. press Ctrl+C to exit.")

	// Block until interrupted
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down bot...")
}
