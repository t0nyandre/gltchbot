package reactionroles

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

const moduleName = "reactionroles"

// ReactionRoles is the module that assigns roles based on message reactions.
// This is a scaffold — full implementation will be added in a future iteration.
type ReactionRoles struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

// New creates a new ReactionRoles module instance.
func New(db *pgxpool.Pool) *ReactionRoles {
	return &ReactionRoles{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (m *ReactionRoles) Name() string        { return moduleName }
func (m *ReactionRoles) Description() string { return "Allow users to assign roles to themselves by reacting to a message" }

// Commands returns the slash commands provided by this module.
// TODO: Implement reaction role management commands.
func (m *ReactionRoles) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		// Placeholder — commands will be added when the module is fully implemented
	}
}

// RegisterHandlers attaches event listeners for this module.
// TODO: Implement messageReactionAdd / messageReactionRemove handlers.
func (m *ReactionRoles) RegisterHandlers(s *discordgo.Session) {
	// s.AddHandler(m.handleReactionAdd)
	// s.AddHandler(m.handleReactionRemove)
}

// OnEnable is called when the module is enabled for a guild.
func (m *ReactionRoles) OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[reactionroles] enabled for guild %s", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *ReactionRoles) OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[reactionroles] disabled for guild %s", guildID)
	return nil
}
