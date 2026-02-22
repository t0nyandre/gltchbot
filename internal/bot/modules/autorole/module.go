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

func (m *AutoRole) Name() string { return moduleName }
func (m *AutoRole) Description() string {
	return "Automatically assign roles to users on join or first activity"
}

// Commands returns the slash commands provided by this module.
func (m *AutoRole) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "autorole",
			Description: "Manage auto-assigned roles",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a role to be automatically assigned",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "The role to automatically assign",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "trigger",
							Description: "When to assign the role",
							Required:    true,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{
									Name:  "On server join",
									Value: "join",
								},
								{
									Name:  "First message in server",
									Value: "first_message",
								},
								{
									Name:  "First reaction in server",
									Value: "first_reaction",
								},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove an auto-assigned role",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "The role to stop automatically assigning",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "trigger",
							Description: "When the role was being assigned",
							Required:    true,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{
									Name:  "On server join",
									Value: "join",
								},
								{
									Name:  "First message in server",
									Value: "first_message",
								},
								{
									Name:  "First reaction in server",
									Value: "first_reaction",
								},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all auto-assigned roles in this server",
				},
			},
		},
	}
}

// RegisterHandlers attaches event listeners for this module.
func (m *AutoRole) RegisterHandlers(s *discordgo.Session) {
	s.AddHandler(m.handleInteraction)
	s.AddHandler(m.handleGuildMemberAdd)
	s.AddHandler(m.handleMessageCreate)
	s.AddHandler(m.handleMessageReactionAdd)
}

// OnEnable is called when the module is enabled for a guild.
func (m *AutoRole) OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[autorole] enabled for guild %s", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *AutoRole) OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[autorole] disabled for guild %s", guildID)
	// Clean up all auto roles and user triggers for this guild
	if err := m.queries.DeleteAllAutoRolesForGuild(ctx, guildID); err != nil {
		log.Printf("[autorole] failed to delete auto roles for guild %s: %v", guildID, err)
		return err
	}
	if err := m.queries.DeleteUserTriggersForGuild(ctx, guildID); err != nil {
		log.Printf("[autorole] failed to delete user triggers for guild %s: %v", guildID, err)
		return err
	}
	return nil
}
