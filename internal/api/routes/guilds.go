package routes

import (
	"encoding/json"
	"net/http"

	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// GuildHandler handles guild-related API routes.
type GuildHandler struct {
	queries *dbsqlc.Queries
}

// NewGuildHandler creates a new GuildHandler.
func NewGuildHandler(queries *dbsqlc.Queries) *GuildHandler {
	return &GuildHandler{queries: queries}
}

// ListGuilds returns all guilds the bot is in.
// GET /api/guilds
func (h *GuildHandler) ListGuilds(w http.ResponseWriter, r *http.Request) {
	guilds, err := h.queries.ListGuilds(r.Context())
	if err != nil {
		jsonError(w, "failed to fetch guilds", http.StatusInternalServerError)
		return
	}
	jsonOK(w, guilds)
}

// GetGuild returns a single guild by ID.
// GET /api/guilds/{guildId}
func (h *GuildHandler) GetGuild(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	guild, err := h.queries.GetGuild(r.Context(), guildID)
	if err != nil {
		jsonError(w, "guild not found", http.StatusNotFound)
		return
	}
	jsonOK(w, guild)
}

// jsonOK writes a 200 JSON response.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
