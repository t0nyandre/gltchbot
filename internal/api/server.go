package api

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/api/middleware"
	"github.com/t0nyandre/gltchbot/internal/api/routes"
	"github.com/t0nyandre/gltchbot/internal/audit"
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

	// Create middleware chain for API routes: auth → audit → routes
	apiHandler := mux
	apiHandler = audit.Middleware(nil)(apiHandler)
	apiHandler = middleware.APIKey(cfg.APIKeys, cfg.OldAPIKeys)(apiHandler)

	// Top-level mux: health is unauthenticated, everything else requires a key
	root := http.NewServeMux()
	root.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Remove server header
		w.Header().Set("Server", "")
		// Prevent caching
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		// Only GET method allowed
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"status":"error","message":"method not allowed"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	root.Handle("/", apiHandler)

	// Apply common middleware to all routes (including /health):
	// logging → recovery → size limit → rate limit → routes
	// Note: Logging is outermost to capture all requests
	// Recovery wraps size/rate limits to catch panics
	// Size limit before rate limit to reject oversized requests early
	commonHandler := root
	commonHandler = middleware.Logging(nil)(commonHandler)
	commonHandler = middleware.Recovery(nil)(commonHandler)
	commonHandler = middleware.SizeLimit(cfg.RequestSizeLimitBytes)(commonHandler)
	commonHandler = middleware.RateLimit(cfg.APIRateLimitGlobal, cfg.APIRateLimitAuth, cfg.APIRateLimitUnauth, cfg.APIRateLimitBurst)(commonHandler)

	// Apply security headers to all routes (including /health)
	securedHandler := middleware.Security(cfg.SecurityHSTSMaxAge, cfg.SecurityCSP, cfg.SecurityPermissionsPolicy)(commonHandler)

	// Apply CORS headers to all routes (including /health)
	corsHandler := middleware.CORS(
		cfg.CORSAllowedOrigins,
		cfg.CORSAllowedMethods,
		cfg.CORSAllowedHeaders,
		cfg.CORSExposedHeaders,
		cfg.CORSMaxAge,
		cfg.CORSAllowCredentials,
	)(securedHandler)

	return &Server{
		cfg:      cfg,
		registry: registry,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.APIPort),
			Handler: corsHandler,
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
