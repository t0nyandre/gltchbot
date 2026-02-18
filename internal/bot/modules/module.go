package modules

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Module is the interface every feature module must implement.
// Modules are self-contained units that register their own slash commands
// and event handlers. They are enabled/disabled per guild.
type Module interface {
	// Name returns the unique identifier for this module (e.g. "jointocreate").
	// Must match the name stored in the modules table.
	Name() string

	// Description returns a human-readable description of the module.
	Description() string

	// Commands returns the slash commands this module provides.
	// These are registered globally or per-guild depending on the environment.
	Commands() []*discordgo.ApplicationCommand

	// RegisterHandlers attaches all event handlers to the Discord session.
	// Called once on startup for all enabled modules.
	RegisterHandlers(s *discordgo.Session)

	// OnEnable is called when the module is enabled for a guild.
	// Use this to create any required database entries for the guild.
	OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error

	// OnDisable is called when the module is disabled for a guild.
	// Use this to clean up guild-specific data if needed.
	OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error
}
