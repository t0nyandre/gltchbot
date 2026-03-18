package routes

import (
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/pagination"
	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
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
	// Parse pagination parameters
	pagination := pagination.ParseQuery(r)
	
	// Get total count
	total, err := h.queries.CountGuilds(r.Context())
	if err != nil {
		response.InternalServerError(w, "failed to count guilds")
		return
	}
	
	// Fetch paginated guilds
	guilds, err := h.queries.ListGuildsPaginated(r.Context(), int32(pagination.Limit), int32(pagination.Offset))
	if err != nil {
		response.InternalServerError(w, "failed to fetch guilds")
		return
	}
	
	// Return paginated response
	pagination.WritePaginatedResponse(w, guilds, int(total), pagination)
}

// GetGuild returns a single guild by ID.
// GET /api/guilds/{guildId}
func (h *GuildHandler) GetGuild(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}
	guild, err := h.queries.GetGuild(r.Context(), guildID)
	if err != nil {
		response.NotFound(w, "guild not found")
		return
	}
	response.OK(w, guild)
}


