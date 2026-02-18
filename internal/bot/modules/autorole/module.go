package autorole

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

const moduleName = "autorole"

// AutoRole is the module that automatically assigns roles to users.
// This is a scaffold — full implementation will be added in a future iteration.
type AutoRole struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

// New creates a new AutoRole module instance.
func New(db *pgxpool.Pool) *AutoRole {
	return &AutoRole{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (m *AutoRole) Name() string        { return moduleName }
func (m *AutoRole) Description() string { return "Automatically assign roles to users on join or first activity" }

// Commands returns the slash commands provided by this module.
// TODO: Implement auto role management commands.
func (m *AutoRole) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		// Placeholder — commands will be added when the module is fully implemented
	}
}

// RegisterHandlers attaches event listeners for this module.
// TODO: Implement guildMemberAdd / messageCreate / messageReactionAdd handlers.
func (m *AutoRole) RegisterHandlers(s *discordgo.Session) {
	// s.AddHandler(m.handleGuildMemberAdd)
	// s.AddHandler(m.handleFirstMessage)
	// s.AddHandler(m.handleFirstReaction)
}

// OnEnable is called when the module is enabled for a guild.
func (m *AutoRole) OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[autorole] enabled for guild %s", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *AutoRole) OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[autorole] disabled for guild %s", guildID)
	return nil
}
