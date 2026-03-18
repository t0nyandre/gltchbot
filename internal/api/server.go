package api

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/api/middleware"
	"github.com/t0nyandre/gltchbot/internal/api/routes"
	"github.com/t0nyandre/gltchbot/internal/bot/modules"
	"github.com/t0nyandre/gltchbot/internal/config"
	"github.com/t0nyandre/gltchbot/internal/logging"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// Server is the HTTP API server.
type Server struct {
	cfg      *config.Config
	server   *http.Server
	registry *modules.Registry
}

// New creates and configures the API server with all routes.
func New(cfg *config.Config, db *pgxpool.Pool, registry *modules.Registry) *Server {
	queries := dbsqlc.New(db)

	// Handlers
	guildHandler := routes.NewGuildHandler(queries)
	moduleHandler := routes.NewModuleHandler(queries, registry)
	jtcHandler := routes.NewJTCHandler(queries)

	mux := http.NewServeMux()

	// --- Guild routes ---
	mux.HandleFunc("GET /api/guilds", guildHandler.ListGuilds)
	mux.HandleFunc("GET /api/guilds/{guildId}", guildHandler.GetGuild)

	// --- Module routes ---
	mux.HandleFunc("GET /api/guilds/{guildId}/modules", moduleHandler.ListGuildModules)
	mux.HandleFunc("GET /api/guilds/{guildId}/modules/{moduleName}", moduleHandler.GetGuildModule)
	mux.HandleFunc("PATCH /api/guilds/{guildId}/modules/{moduleName}", moduleHandler.UpdateGuildModule)

	// --- JoinToCreate module routes ---
	// Note: this must come before the generic {moduleName} route above to avoid conflicts,
	// but since we use distinct methods (GET vs GET with extra path segment) Go 1.22 handles it.
	mux.HandleFunc("GET /api/guilds/{guildId}/modules/jointocreate", jtcHandler.GetJTCConfig)
	mux.HandleFunc("POST /api/guilds/{guildId}/modules/jointocreate/parents", jtcHandler.AddParentChannel)
	mux.HandleFunc("DELETE /api/guilds/{guildId}/modules/jointocreate/parents/{channelId}", jtcHandler.DeleteParentChannel)

	// Create middleware chain: recovery → logging → auth → routes
	handler := mux
	handler = middleware.APIKey(cfg.APIKey)(handler)
	handler = middleware.Logging(nil)(handler)
	handler = middleware.Recovery(nil)(handler)

	// Top-level mux: health is unauthenticated, everything else requires a key
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	root.Handle("/", handler)

	return &Server{
		cfg:      cfg,
		registry: registry,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.APIPort),
			Handler: root,
		},
	}
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	logging.Info("API server starting", "port", s.cfg.APIPort)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	return s.server.Close()
}
