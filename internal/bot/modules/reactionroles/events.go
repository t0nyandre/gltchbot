package reactionroles

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// handleReactionAdd is called when a user adds a reaction to a message.
func (m *ReactionRoles) handleReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Ignore reactions from bots
	if r.Member == nil || r.Member.User.Bot {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: r.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Get the reaction role for this message and emoji
	emoji := formatEmojiForStorage(r.Emoji)
	reactionRole, err := m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
		MessageID: r.MessageID,
		Emoji:     emoji,
	})
	if err != nil {
		// No reaction role configured for this message+emoji
		return
	}

	// Add the role to the user
	err = s.GuildMemberRoleAdd(r.GuildID, r.UserID, reactionRole.RoleID)
	if err != nil {
		log.Printf("[reactionroles] failed to add role %s to user %s: %v", reactionRole.RoleID, r.UserID, err)
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
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: r.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Get the reaction role for this message and emoji
	emoji := formatEmojiForStorage(r.Emoji)
	reactionRole, err := m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
		MessageID: r.MessageID,
		Emoji:     emoji,
	})
	if err != nil {
		// No reaction role configured for this message+emoji
		return
	}

	// Remove the role from the user
	err = s.GuildMemberRoleRemove(r.GuildID, r.UserID, reactionRole.RoleID)
	if err != nil {
		log.Printf("[reactionroles] failed to remove role %s from user %s: %v", reactionRole.RoleID, r.UserID, err)
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
