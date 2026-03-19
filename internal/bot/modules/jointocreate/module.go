package jointocreate

import (
	"context"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/cache"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

const moduleName = "jointocreate"

// JoinToCreate is the module that automatically creates temporary voice channels.
type JoinToCreate struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
	cache   *cache.TTL[string, *dbsqlc.JtcParentChannel]
}

// New creates a new JoinToCreate module instance.
func New(db *pgxpool.Pool) *JoinToCreate {
	// Read cache TTL from environment variable, default 5 minutes (300 seconds)
	ttlSeconds := 300
	if env := os.Getenv("CACHE_TTL_SECONDS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 && v <= math.MaxInt32 {
			ttlSeconds = v
		}
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	cache := cache.New[string, *dbsqlc.JtcParentChannel](ttl)
	// Start background cleanup (optional, but recommended for long-running bot)
	cache.StartCleanup()
	return &JoinToCreate{
		db:      db,
		queries: dbsqlc.New(db),
		cache:   cache,
	}
}

func (m *JoinToCreate) Name() string { return moduleName }
func (m *JoinToCreate) Description() string {
	return "Automatically create temporary voice channels when a user joins a designated parent channel"
}

// getParentChannel retrieves a JTC parent channel from cache or database.
// Returns a pointer to the parent channel if found and is a parent channel,
// nil if the channel is not a parent channel (cached or determined from DB).
func (m *JoinToCreate) getParentChannel(ctx context.Context, channelID string) *dbsqlc.JtcParentChannel {
	// Try cache first
	if cached, ok := m.cache.Get(channelID); ok {
		logging.Debug("cache hit for parent channel", "module", "jointocreate", "channel_id", channelID)
		return cached
	}
	logging.Debug("cache miss for parent channel", "module", "jointocreate", "channel_id", channelID)
	// Query database
	parent, err := m.queries.GetJTCParentChannel(ctx, channelID)
	if err != nil {
		// Channel is not a parent channel. Cache nil to avoid repeated DB queries.
		// Use a shorter TTL for negative caching (e.g., 30 seconds) to allow for new parent channels.
		// We'll reuse default TTL but can be separate; for simplicity, use same TTL.
		m.cache.Set(channelID, nil)
		return nil
	}
	// Store in cache with default TTL
	m.cache.Set(channelID, &parent)
	return &parent
}

// invalidateParentChannel removes a channel from the cache.
func (m *JoinToCreate) invalidateParentChannel(channelID string) {
	m.cache.Delete(channelID)
	logging.Debug("invalidated parent channel cache", "module", "jointocreate", "channel_id", channelID)
}

// invalidateAllGuildParentChannels removes all cached parent channels for a guild.
func (m *JoinToCreate) invalidateAllGuildParentChannels(guildID string) {
	m.cache.DeleteIf(func(_ string, parent *dbsqlc.JtcParentChannel) bool {
		return parent != nil && parent.GuildID == guildID
	})
	logging.Debug("invalidated parent channel cache for guild", "module", "jointocreate", "guild_id", guildID)
}

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
func (m *JoinToCreate) OnEnable(ctx context.Context, guildID string) error {
	logging.Info("enabled for guild", "module", "jointocreate", "guild_id", guildID)
	return nil
}

// OnDisable is called when the module is disabled for a guild.
func (m *JoinToCreate) OnDisable(ctx context.Context, guildID string) error {
	logging.Info("disabled for guild", "module", "jointocreate", "guild_id", guildID)
	// Invalidate all cached parent channels for this guild
	m.invalidateAllGuildParentChannels(guildID)
	return nil
}
