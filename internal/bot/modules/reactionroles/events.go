package reactionroles

import (
	"context"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
	"github.com/t0nyandre/gltchbot/internal/db"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// handleReactionAdd is called when a user adds a reaction to a message.
func (m *ReactionRoles) handleReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Ignore reactions from bots
	if r.Member == nil || r.Member.User.Bot {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	var enabled bool
	err := db.WithRetry(ctx, func(ctx context.Context) error {
		var innerErr error
		enabled, innerErr = m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
			GuildID: r.GuildID,
			Name:    moduleName,
		})
		return innerErr
	}, db.DefaultRetryConfig())
	if err != nil || !enabled {
		return
	}

	// Get the reaction role for this message and emoji
	emoji := formatEmojiForStorage(r.Emoji)
	var reactionRole dbsqlc.ReactionRole
	err = db.WithRetry(ctx, func(ctx context.Context) error {
		var innerErr error
		reactionRole, innerErr = m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
			MessageID: r.MessageID,
			Emoji:     emoji,
		})
		return innerErr
	}, db.DefaultRetryConfig())
	if err != nil {
		// No reaction role configured for this message+emoji
		return
	}

	// Add the role to the user
	err = s.GuildMemberRoleAdd(r.GuildID, r.UserID, reactionRole.RoleID)
	if err != nil {
		logging.Error("failed to add role to user", "module", "reactionroles", "role_id", reactionRole.RoleID, "user_id", r.UserID, "error", err)
		// Try to remove the reaction since we couldn't add the role
		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, emoji, r.UserID)
	}
}

// handleReactionRemove is called when a user removes a reaction from a message.
func (m *ReactionRoles) handleReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	// Ignore if we can't get member info (shouldn't happen for reaction remove)
	if r.UserID == "" {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	var enabled bool
	err := db.WithRetry(ctx, func(ctx context.Context) error {
		var innerErr error
		enabled, innerErr = m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
			GuildID: r.GuildID,
			Name:    moduleName,
		})
		return innerErr
	}, db.DefaultRetryConfig())
	if err != nil || !enabled {
		return
	}

	// Get the reaction role for this message and emoji
	emoji := formatEmojiForStorage(r.Emoji)
	var reactionRole dbsqlc.ReactionRole
	err = db.WithRetry(ctx, func(ctx context.Context) error {
		var innerErr error
		reactionRole, innerErr = m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
			MessageID: r.MessageID,
			Emoji:     emoji,
		})
		return innerErr
	}, db.DefaultRetryConfig())
	if err != nil {
		// No reaction role configured for this message+emoji
		return
	}

	// Remove the role from the user
	err = s.GuildMemberRoleRemove(r.GuildID, r.UserID, reactionRole.RoleID)
	if err != nil {
		logging.Error("failed to remove role from user", "module", "reactionroles", "role_id", reactionRole.RoleID, "user_id", r.UserID, "error", err)
	}
}

// formatEmojiForStorage converts a discordgo.Emoji to a string format for storage/lookup.
// This matches the format used in parseEmoji from commands.go.
func formatEmojiForStorage(emoji discordgo.Emoji) string {
	// Custom emoji
	if emoji.ID != "" {
		return emoji.Name + ":" + emoji.ID
	}

	// Unicode emoji
	return emoji.Name
}
