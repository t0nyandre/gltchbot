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
func (m *ReactionRoles) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "reactionrole",
			Description: "Manage reaction roles",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a reaction role to a message",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "message_id",
							Description: "The ID of the message to add the reaction role to",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "emoji",
							Description: "The emoji to use (unicode or custom emoji like :emoji:)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "The role to assign when this emoji is reacted",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a reaction role from a message",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "message_id",
							Description: "The ID of the message to remove the reaction role from",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "emoji",
							Description: "The emoji to remove (unicode or custom emoji like :emoji:)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all reaction roles in this server",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "fix",
					Description: "Fix reactions on a message (remove all and re-add missing ones)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "message_id",
							Description: "The ID of the message to fix reactions for",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

// RegisterHandlers attaches event listeners for this module.
func (m *ReactionRoles) RegisterHandlers(s *discordgo.Session) {
	s.AddHandler(m.handleInteraction)
	s.AddHandler(m.handleReactionAdd)
	s.AddHandler(m.handleReactionRemove)
}

// OnEnable is called when the module is enabled for a guild.
func (m *ReactionRoles) OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[reactionroles] enabled for guild %s", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *ReactionRoles) OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[reactionroles] disabled for guild %s", guildID)
	// Clean up all reaction roles for this guild when module is disabled
	if err := m.queries.DeleteAllReactionRolesForGuild(ctx, guildID); err != nil {
		log.Printf("[reactionroles] error cleaning up reaction roles for guild %s: %v", guildID, err)
	}
	return nil
}
