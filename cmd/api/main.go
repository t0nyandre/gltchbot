package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/t0nyandre/gltchbot/internal/api"
	"github.com/t0nyandre/gltchbot/internal/bot/modules"
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

	// Connect to PostgreSQL (migrations are run by the bot on startup,
	// but we run them here too in case the API starts first)
	pool, err := db.New(ctx, cfg)
	if err != nil {
		logging.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create a registry without a session (API only needs DB access for module management)
	registry := modules.NewRegistry(pool)

	// Register all available modules (same as in the bot)
	for _, module := range modules.DefaultModules(pool) {
		registry.Register(module)
	}

	// Create and start the API server
	server := api.New(cfg, pool, registry)

	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Fatalf("API server error: %v", err)
		}
	}()

	logging.Info("API is running. press Ctrl+C to exit.")

	// Block until interrupted
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logging.Info("shutting down API...")
	if err := server.Shutdown(); err != nil {
		logging.Error("error shutting down API server", "error", err)
	}
}
