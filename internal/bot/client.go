package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/bot/modules"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/autorole"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/jointocreate"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/reactionroles"
	"github.com/t0nyandre/gltchbot/internal/config"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// Client wraps the discordgo session and the module registry.
type Client struct {
	Session  *discordgo.Session
	Registry *modules.Registry
	cfg      *config.Config
	db       *pgxpool.Pool
	queries  *dbsqlc.Queries
}

// New creates and configures a new Discord bot client.
func New(cfg *config.Config, db *pgxpool.Pool) (*Client, error) {
	s, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	// Request the intents we need
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsGuildMembers

	registry := modules.NewRegistry(db)

	client := &Client{
		Session:  s,
		Registry: registry,
		cfg:      cfg,
		db:       db,
		queries:  dbsqlc.New(db),
	}

	// Register all available modules
	registry.Register(jointocreate.New(db))
	registry.Register(reactionroles.New(db))
	registry.Register(autorole.New(db))

	return client, nil
}

// Start opens the Discord gateway connection and sets the bot's presence.
func (c *Client) Start(ctx context.Context) error {
	// Register core event handlers
	c.Session.AddHandler(c.onReady)
	c.Session.AddHandler(c.onGuildCreate)
	c.Session.AddHandler(c.onInteractionCreate)

	// Register all module event handlers
	c.Registry.RegisterHandlers(c.Session)

	// Open the websocket connection
	if err := c.Session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	// Register slash commands immediately after connecting so they are always
	// up to date on every startup — not deferred to the Ready event, which
	// is not guaranteed to fire again on reconnects.
	if err := c.Registry.RegisterCommands(c.Session, c.cfg.DiscordAppID, c.cfg.DiscordDevGuildID); err != nil {
		log.Printf("failed to register commands: %v", err)
	}

	return nil
}

// Close gracefully shuts down the Discord session.
func (c *Client) Close() {
	if err := c.Session.Close(); err != nil {
		log.Printf("error closing discord session: %v", err)
	}
}

// onReady is called when the bot successfully connects to Discord.
func (c *Client) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("bot is ready: %s#%s (ID: %s)", r.User.Username, r.User.Discriminator, r.User.ID)

	// Set bot presence from config
	if err := s.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: c.cfg.BotStatus,
		Activities: []*discordgo.Activity{
			{
				Name: c.cfg.BotActivityText,
				Type: c.cfg.BotActivityType,
			},
		},
	}); err != nil {
		log.Printf("failed to set bot presence: %v", err)
	}
}

// onGuildCreate fires when the bot joins a guild or on startup for all guilds.
// We use it to ensure the guild exists in our database.
func (c *Client) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	ctx := context.Background()
	if _, err := c.queries.UpsertGuild(ctx, dbsqlc.UpsertGuildParams{
		ID:   g.ID,
		Name: g.Name,
	}); err != nil {
		log.Printf("failed to upsert guild %s: %v", g.ID, err)
	}
}

// onInteractionCreate handles incoming slash command interactions and routes
// them to the correct module command handler.
func (c *Client) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	name := i.ApplicationCommandData().Name
	for _, m := range c.Registry.All() {
		for _, cmd := range m.Commands() {
			if cmd.Name == name {
				// The module's own handler picks this up via AddHandler;
				// this router is only for commands that modules don't self-handle.
				return
			}
		}
	}
}
