package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/bot/modules"
	"github.com/t0nyandre/gltchbot/internal/bot/ratelimit"
	"github.com/t0nyandre/gltchbot/internal/config"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// Client wraps the discordgo session and the module registry.
type Client struct {
	Session  *ratelimit.RateLimitedSession
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

	// Wrap session with rate limiting
	wrappedSession := ratelimit.NewRateLimitedSession(s)

	registry := modules.NewRegistry(db)

	client := &Client{
		Session:  wrappedSession,
		Registry: registry,
		cfg:      cfg,
		db:       db,
		queries:  dbsqlc.New(db),
	}

	// Register all available modules
	for _, module := range modules.DefaultModules(db) {
		registry.Register(module)
	}

	return client, nil
}

// Start opens the Discord gateway connection and sets the bot's presence.
func (c *Client) Start(ctx context.Context) error {
	// Register core event handlers
	c.Session.AddHandler(c.onReady)
	c.Session.AddHandler(c.onGuildCreate)

	// Register all module event handlers
	c.Registry.RegisterHandlers(c.Session.Session)

	// Open the websocket connection
	if err := c.Session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	// Register slash commands immediately after connecting so they are always
	// up to date on every startup — not deferred to the Ready event, which
	// is not guaranteed to fire again on reconnects.
	if err := c.Registry.RegisterCommands(c.Session.Session, c.cfg.DiscordAppID, c.cfg.DiscordDevGuildID); err != nil {
		logging.Error("failed to register commands", "error", err)
	}

	return nil
}

// Close gracefully shuts down the Discord session.
func (c *Client) Close() {
	if err := c.Session.Close(); err != nil {
		logging.Error("error closing discord session", "error", err)
	}
}

// onReady is called when the bot successfully connects to Discord.
func (c *Client) onReady(s *discordgo.Session, r *discordgo.Ready) {
	logging.Info("bot is ready", "username", r.User.Username, "discriminator", r.User.Discriminator, "user_id", r.User.ID)

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
		logging.Error("failed to set bot presence", "error", err)
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
		logging.Error("failed to upsert guild", "guild_id", g.ID, "error", err)
	}
}
