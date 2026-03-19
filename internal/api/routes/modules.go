package routes

import (
	"encoding/json"
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
	"github.com/t0nyandre/gltchbot/internal/audit"
	"github.com/t0nyandre/gltchbot/internal/bot/modules"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// ModuleHandler handles module-related API routes.
type ModuleHandler struct {
	queries  *dbsqlc.Queries
	registry *modules.Registry
}

// NewModuleHandler creates a new ModuleHandler.
func NewModuleHandler(queries *dbsqlc.Queries, registry *modules.Registry) *ModuleHandler {
	return &ModuleHandler{
		queries:  queries,
		registry: registry,
	}
}

// ListGuildModules returns all modules with their enabled status for a guild.
// GET /api/guilds/{guildId}/modules
func (h *ModuleHandler) ListGuildModules(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}
	// Audit log: sensitive data read
	audit.LogEvent(r.Context(), audit.EventSensitiveDataRead, validation.SanitizeLogDetails(map[string]any{
		"guild_id": guildID,
		"resource": "guild_modules",
	}))
	mods, err := h.queries.ListGuildModules(r.Context(), guildID)
	if err != nil {
		response.InternalServerError(w, "failed to fetch modules")
		return
	}
	response.OK(w, mods)
}

// GetGuildModule returns a single module's status for a guild.
// GET /api/guilds/{guildId}/modules/{moduleName}
func (h *ModuleHandler) GetGuildModule(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}
	moduleName := r.PathValue("moduleName")
	if err := validation.ValidateModuleName(moduleName); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	// Audit log: sensitive data read
	audit.LogEvent(r.Context(), audit.EventSensitiveDataRead, validation.SanitizeLogDetails(map[string]any{
		"guild_id": guildID,
		"module":   moduleName,
		"resource": "guild_module",
	}))

	mod, err := h.queries.GetGuildModule(r.Context(), dbsqlc.GetGuildModuleParams{
		GuildID: guildID,
		Name:    moduleName,
	})
	if err != nil {
		response.NotFound(w, "module not found")
		return
	}
	response.OK(w, mod)
}

// UpdateGuildModule enables or disables a module for a guild.
// PATCH /api/guilds/{guildId}/modules/{moduleName}
func (h *ModuleHandler) UpdateGuildModule(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("guildId")
	if err := validation.ValidateGuildID(guildID); err != nil {
		response.BadRequest(w, "invalid guild ID: "+err.Error())
		return
	}
	moduleName := r.PathValue("moduleName")
	if err := validation.ValidateModuleName(moduleName); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	ctx := r.Context()

	if body.Enabled {
		if err := h.registry.EnableForGuild(ctx, moduleName, guildID); err != nil {
			response.InternalServerError(w, "failed to enable module: "+err.Error())
			return
		}
		// Audit log
		audit.LogEvent(ctx, audit.EventModuleEnabled, validation.SanitizeLogDetails(map[string]any{
			"guild_id": guildID,
			"module":   moduleName,
		}))
	} else {
		if err := h.registry.DisableForGuild(ctx, moduleName, guildID); err != nil {
			response.InternalServerError(w, "failed to disable module: "+err.Error())
			return
		}
		// Audit log
		audit.LogEvent(ctx, audit.EventModuleDisabled, validation.SanitizeLogDetails(map[string]any{
			"guild_id": guildID,
			"module":   moduleName,
		}))
	}

	response.OK(w, map[string]any{"guild_id": guildID, "module": moduleName, "enabled": body.Enabled})
}
