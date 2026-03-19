package routes

import (
	"encoding/json"
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/pagination"
	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
	"github.com/t0nyandre/gltchbot/internal/audit"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// JTCHandler handles JoinToCreate-specific API routes.
type JTCHandler struct {
	queries *dbsqlc.Queries
}

// NewJTCHandler creates a new JTCHandler.
func NewJTCHandler(queries *dbsqlc.Queries) *JTCHandler {
	return &JTCHandler{queries: queries}
}

// GetJTCConfig returns all JoinToCreate parent channels for a guild.
// GET /api/guilds/{guildId}/modules/jointocreate
func (h *JTCHandler) GetJTCConfig(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}

	// Audit log: sensitive data read
	audit.LogEvent(r.Context(), audit.EventSensitiveDataRead, validation.SanitizeLogDetails(map[string]any{
		"guild_id": guildID,
		"resource": "jtc_config",
	}))

	// Parse pagination parameters
	paginationParams := pagination.ParseQuery(r)

	// Convert pagination parameters to int32 with bounds checking
	limit32, err := validation.SafeInt32(paginationParams.Limit)
	if err != nil {
		response.InternalServerError(w, "invalid pagination limit")
		return
	}
	offset32, err := validation.SafeInt32(paginationParams.Offset)
	if err != nil {
		response.InternalServerError(w, "invalid pagination offset")
		return
	}

	// Get total count for this guild
	total, err := h.queries.CountJTCParentChannels(r.Context(), guildID)
	if err != nil {
		response.InternalServerError(w, "failed to count JTC parent channels")
		return
	}

	// Fetch paginated parent channels
	parents, err := h.queries.ListJTCParentChannelsPaginated(r.Context(), guildID, limit32, offset32)
	if err != nil {
		response.InternalServerError(w, "failed to fetch JTC config")
		return
	}

	// Convert total count to int with bounds checking
	totalInt, err := validation.SafeInt(total)
	if err != nil {
		response.InternalServerError(w, "total count too large")
		return
	}

	// Build response with pagination metadata
	paginatedResp := pagination.NewResponse(parents, totalInt, paginationParams)
	// Add guild_id to the response
	fullResp := map[string]any{
		"guild_id":        guildID,
		"parent_channels": paginatedResp.Data,
		"pagination":      paginatedResp.Pagination,
	}
	response.OK(w, fullResp)
}

// AddParentChannel adds a new JTC parent channel entry (via API, not Discord command).
// POST /api/guilds/{guildId}/modules/jointocreate/parents
func (h *JTCHandler) AddParentChannel(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}

	var body struct {
		ChannelID   string `json:"channel_id"`
		CategoryID  string `json:"category_id"`
		ChannelName string `json:"channel_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	// Validate required fields
	if err := validation.ValidateRequiredString("channel_id", body.ChannelID); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := validation.ValidateRequiredString("category_id", body.CategoryID); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := validation.ValidateRequiredString("channel_name", body.ChannelName); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	// Validate Discord IDs
	if err := validation.ValidateChannelID(body.ChannelID); err != nil {
		response.BadRequest(w, "invalid channel ID: "+err.Error())
		return
	}
	if err := validation.ValidateDiscordID(body.CategoryID); err != nil {
		response.BadRequest(w, "invalid category ID: "+err.Error())
		return
	}

	// Validate channel name length (max 100 characters as per Discord)
	if err := validation.ValidateMaxLength("channel_name", body.ChannelName, 100); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	parent, err := h.queries.CreateJTCParentChannel(r.Context(), dbsqlc.CreateJTCParentChannelParams{
		GuildID:     guildID,
		ChannelID:   body.ChannelID,
		CategoryID:  body.CategoryID,
		ChannelName: body.ChannelName,
	})
	if err != nil {
		response.InternalServerError(w, "failed to create parent channel")
		return
	}

	// Audit log: sensitive data write
	audit.LogEvent(r.Context(), audit.EventSensitiveDataWrite, validation.SanitizeLogDetails(map[string]any{
		"guild_id":   guildID,
		"channel_id": body.ChannelID,
		"resource":   "jtc_parent_channel",
		"action":     "create",
	}))

	response.Created(w, parent)
}

// DeleteParentChannel removes a JTC parent channel entry.
// DELETE /api/guilds/{guildId}/modules/jointocreate/parents/{channelId}
func (h *JTCHandler) DeleteParentChannel(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}
	channelID := r.PathValue("channelId")
	if err := validation.ValidateChannelID(channelID); err != nil {
		response.BadRequest(w, "invalid channel ID: "+err.Error())
		return
	}

	if err := h.queries.DeleteJTCParentChannel(r.Context(), dbsqlc.DeleteJTCParentChannelParams{
		ChannelID: channelID,
		GuildID:   guildID,
	}); err != nil {
		response.InternalServerError(w, "failed to delete parent channel")
		return
	}

	// Audit log: sensitive data write
	audit.LogEvent(r.Context(), audit.EventSensitiveDataWrite, validation.SanitizeLogDetails(map[string]any{
		"guild_id":   guildID,
		"channel_id": channelID,
		"resource":   "jtc_parent_channel",
		"action":     "delete",
	}))

	response.NoContent(w)
}
