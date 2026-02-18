package jointocreate

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

const moduleName = "jointocreate"

// JoinToCreate is the module that automatically creates temporary voice channels.
type JoinToCreate struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

// New creates a new JoinToCreate module instance.
func New(db *pgxpool.Pool) *JoinToCreate {
	return &JoinToCreate{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (m *JoinToCreate) Name() string        { return moduleName }
func (m *JoinToCreate) Description() string { return "Automatically create temporary voice channels when a user joins a designated parent channel" }

// Commands returns the slash commands provided by this module.
func (m *JoinToCreate) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "jointocreate",
			Description: "Manage JoinToCreate parent channels",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "setup",
					Description: "Set up a new JoinToCreate parent channel in a category",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "category",
							Description: "The category to create the channel in",
							Required:    true,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildCategory,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "channel_name",
							Description: "Name for the JoinToCreate trigger channel (e.g. '+ Create Channel')",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a JoinToCreate parent channel",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "The JoinToCreate parent channel to remove",
							Required:    true,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildVoice,
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List all JoinToCreate parent channels in this server",
				},
			},
		},
	}
}

// RegisterHandlers attaches all event listeners for this module.
func (m *JoinToCreate) RegisterHandlers(s *discordgo.Session) {
	s.AddHandler(m.handleInteraction)
	s.AddHandler(m.handleVoiceStateUpdate)
	s.AddHandler(m.handleChannelUpdate)
	s.AddHandler(m.handleChannelDelete)
}

// OnEnable is called when the module is enabled for a guild.
func (m *JoinToCreate) OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[jointocreate] enabled for guild %s", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *JoinToCreate) OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error {
	log.Printf("[jointocreate] disabled for guild %s", guildID)
	return nil
}
