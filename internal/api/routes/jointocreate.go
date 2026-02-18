package routes

import (
	"encoding/json"
	"net/http"

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
	parents, err := h.queries.ListJTCParentChannels(r.Context(), guildID)
	if err != nil {
		jsonError(w, "failed to fetch JTC config", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"guild_id": guildID, "parent_channels": parents})
}

// AddParentChannel adds a new JTC parent channel entry (via API, not Discord command).
// POST /api/guilds/{guildId}/modules/jointocreate/parents
func (h *JTCHandler) AddParentChannel(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")

	var body struct {
		ChannelID   string `json:"channel_id"`
		CategoryID  string `json:"category_id"`
		ChannelName string `json:"channel_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.ChannelID == "" || body.CategoryID == "" || body.ChannelName == "" {
		jsonError(w, "channel_id, category_id, and channel_name are required", http.StatusBadRequest)
		return
	}

	parent, err := h.queries.CreateJTCParentChannel(r.Context(), dbsqlc.CreateJTCParentChannelParams{
		GuildID:     guildID,
		ChannelID:   body.ChannelID,
		CategoryID:  body.CategoryID,
		ChannelName: body.ChannelName,
	})
	if err != nil {
		jsonError(w, "failed to create parent channel", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(parent)
}

// DeleteParentChannel removes a JTC parent channel entry.
// DELETE /api/guilds/{guildId}/modules/jointocreate/parents/{channelId}
func (h *JTCHandler) DeleteParentChannel(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	channelID := r.PathValue("channelId")

	if err := h.queries.DeleteJTCParentChannel(r.Context(), dbsqlc.DeleteJTCParentChannelParams{
		ChannelID: channelID,
		GuildID:   guildID,
	}); err != nil {
		jsonError(w, "failed to delete parent channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
