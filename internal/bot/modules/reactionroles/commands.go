package reactionroles

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// logStructured logs a structured message with consistent fields
func logStructured(level, module, operation, guildID, userID, message string, data map[string]interface{}) {
	// Build key-value pairs for structured logging
	args := make([]any, 0)
	args = append(args, "module", module, "operation", operation)
	if guildID != "" {
		args = append(args, "guild_id", guildID)
	}
	if userID != "" {
		args = append(args, "user_id", userID)
	}
	if message != "" {
		args = append(args, "message", message)
	}
	for key, value := range data {
		args = append(args, key, value)
	}
	
	// Call appropriate logging function based on level
	switch strings.ToUpper(level) {
	case "DEBUG":
		logging.Debug(operation, args...)
	case "INFO":
		logging.Info(operation, args...)
	case "WARN":
		logging.Warn(operation, args...)
	case "ERROR":
		logging.Error(operation, args...)
	default:
		logging.Info(operation, args...)
	}
}

// logCommandStart logs the start of a command execution
func logCommandStart(command, guildID, userID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["command"] = command
	logStructured("INFO", "reactionroles", "command_start", guildID, userID, "", data)
}

// logCommandEnd logs the end of a command execution with duration
func logCommandEnd(command, guildID, userID string, duration time.Duration, success bool, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["command"] = command
	data["duration_ms"] = duration.Milliseconds()
	data["success"] = success
	logStructured("INFO", "reactionroles", "command_end", guildID, userID, "", data)
}

// logDebug logs debug information
func logDebug(operation, guildID, userID, message string, data map[string]interface{}) {
	logStructured("DEBUG", "reactionroles", operation, guildID, userID, message, data)
}

// logError logs an error with context
func logError(operation, guildID, userID, message string, err error, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["error"] = err.Error()
	logStructured("ERROR", "reactionroles", operation, guildID, userID, message, data)
}

// deferResponse sends a deferred response and logs it
func deferResponse(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if err != nil {
		logError("defer_response", i.GuildID, i.Member.User.ID, "failed to send deferred response", err, map[string]interface{}{
			"command": i.ApplicationCommandData().Name,
		})
		return err
	}

	logDebug("defer_response", i.GuildID, i.Member.User.ID, "deferred response sent", map[string]interface{}{
		"command": i.ApplicationCommandData().Name,
	})
	return nil
}

// editDeferredResponse edits a deferred response with the final result
func editDeferredResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})

	if err != nil {
		logError("edit_deferred_response", i.GuildID, i.Member.User.ID, "failed to edit deferred response", err, map[string]interface{}{
			"command":        i.ApplicationCommandData().Name,
			"content_length": len(content),
		})
		return err
	}

	logDebug("edit_deferred_response", i.GuildID, i.Member.User.ID, "deferred response edited", map[string]interface{}{
		"command":        i.ApplicationCommandData().Name,
		"content_length": len(content),
	})
	return nil
}

// respondEphemeral sends an ephemeral (only visible to the user) response.
func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleInteraction routes slash command interactions for this module.
func (m *ReactionRoles) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	startTime := time.Now()

	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "reactionrole" {
		return
	}

	// Must be used in a guild
	if i.GuildID == "" {
		respondEphemeral(s, i, "This command can only be used in a server.")
		return
	}

	// Get user ID for logging
	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Log command start
	logCommandStart("reactionrole", i.GuildID, userID, map[string]interface{}{
		"subcommand": i.ApplicationCommandData().Options[0].Name,
	})

	// Require administrator permission
	if i.Member == nil || i.Member.Permissions&discordgo.PermissionAdministrator == 0 {
		respondEphemeral(s, i, "❌ You need the **Administrator** permission to use this command.")
		logCommandEnd("reactionrole", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"subcommand": i.ApplicationCommandData().Options[0].Name,
			"error":      "insufficient_permissions",
		})
		return
	}

	// Check if module is enabled for this guild
	ctx := context.Background()
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: i.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		respondEphemeral(s, i, "❌ The Reaction Roles module is not enabled for this server. Please enable it first via the dashboard.")
		logCommandEnd("reactionrole", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"subcommand": i.ApplicationCommandData().Options[0].Name,
			"error":      "module_disabled",
		})
		return
	}

	sub := i.ApplicationCommandData().Options[0]
	switch sub.Name {
	case "add":
		m.handleAdd(s, i, sub)
	case "remove":
		m.handleRemove(s, i, sub)
	case "list":
		m.handleList(s, i)
	case "fix":
		m.handleFix(s, i, sub)
	}

	// Log successful command routing
	logCommandEnd("reactionrole", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"subcommand": sub.Name,
	})
}

// handleAdd creates a new reaction role for a message.
func (m *ReactionRoles) handleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	startTime := time.Now()
	ctx := context.Background()

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Send deferred response immediately to avoid timeout
	if err := deferResponse(s, i); err != nil {
		// If we can't send deferred response, log and return (user will see "application did not respond")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "defer_response_failed",
		})
		return
	}

	var messageID, emoji string
	var role *discordgo.Role
	for _, opt := range sub.Options {
		switch opt.Name {
		case "message_id":
			messageID = opt.StringValue()
		case "emoji":
			emoji = opt.StringValue()
		case "role":
			role = opt.RoleValue(s, i.GuildID)
		}
	}

	if role == nil {
		_ = editDeferredResponse(s, i, "❌ Invalid role selected.")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "invalid_role",
		})
		return
	}

	// Try to fetch the message to get channel ID and verify it exists
	msg, err := s.ChannelMessage(i.ChannelID, messageID)
	if err != nil {
		logError("add_operation", i.GuildID, userID, "failed to fetch message", err, map[string]interface{}{
			"message_id": messageID,
			"channel_id": i.ChannelID,
		})
		_ = editDeferredResponse(s, i, "❌ Could not find the specified message. Make sure the message ID is correct and the message is in this channel.")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "message_not_found",
		})
		return
	}

	// Parse emoji to get the proper format for reacting
	emojiClean := parseEmoji(emoji)
	if emojiClean == "" {
		_ = editDeferredResponse(s, i, "❌ Invalid emoji format. Use:\n• Unicode emoji: ✅ 🔥 🎉\n• Custom emoji: `:emoji_name:` or `:emoji_name:id`\n• You can also copy-paste emojis directly from Discord.")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"emoji_input": emoji,
			"error":       "invalid_emoji_format",
		})
		return
	}

	// Check if reaction role already exists for this message+emoji
	existing, err := m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
		MessageID: messageID,
		Emoji:     emojiClean,
	})
	if err == nil && existing.ID > 0 {
		_ = editDeferredResponse(s, i, fmt.Sprintf("❌ A reaction role with emoji %s already exists for this message.", emojiClean))
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"emoji":      emojiClean,
			"error":      "reaction_role_exists",
		})
		return
	}

	// Create reaction role in database
	reactionRole, err := m.queries.CreateReactionRole(ctx, dbsqlc.CreateReactionRoleParams{
		GuildID:   i.GuildID,
		ChannelID: msg.ChannelID,
		MessageID: messageID,
		Emoji:     emojiClean,
		RoleID:    role.ID,
	})
	if err != nil {
		logError("add_operation", i.GuildID, userID, "failed to create reaction role in database", err, map[string]interface{}{
			"message_id": messageID,
			"emoji":      emojiClean,
			"role_id":    role.ID,
		})
		_ = editDeferredResponse(s, i, "❌ Failed to create reaction role in database.")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "database_create_failed",
		})
		return
	}

	// Add the reaction to the message
	err = s.MessageReactionAdd(msg.ChannelID, msg.ID, emojiClean)
	if err != nil {
		logError("add_operation", i.GuildID, userID, "failed to add reaction to message", err, map[string]interface{}{
			"message_id": messageID,
			"emoji":      emojiClean,
			"channel_id": msg.ChannelID,
		})
		// Clean up database entry if we can't add the reaction
		_ = m.queries.DeleteReactionRole(ctx, reactionRole.ID)
		_ = editDeferredResponse(s, i, "❌ Failed to add reaction to the message. Make sure:\n• I have the **Add Reactions** permission\n• The emoji is from this server or a Unicode emoji\n• The emoji still exists in this server")
		logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "reaction_add_failed",
		})
		return
	}

	// Build success message
	successMsg := fmt.Sprintf("✅ Reaction role created! Reacting with %s on [this message](https://discord.com/channels/%s/%s/%s) will now give the <@&%s> role.",
		emojiClean, i.GuildID, msg.ChannelID, msg.ID, role.ID)

	// Send final result
	if err := editDeferredResponse(s, i, successMsg); err != nil {
		// Log error but command still executed successfully
		logError("add_completion", i.GuildID, userID, "failed to send final response", err, map[string]interface{}{
			"message_id": messageID,
		})
	}

	// Log command completion
	logCommandEnd("reactionrole_add", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"message_id":       messageID,
		"emoji":            emojiClean,
		"role_id":          role.ID,
		"channel_id":       msg.ChannelID,
		"reaction_role_id": reactionRole.ID,
	})
}

// handleRemove deletes a reaction role from a message.
func (m *ReactionRoles) handleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	ctx := context.Background()

	var messageID, emoji string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "message_id":
			messageID = opt.StringValue()
		case "emoji":
			emoji = opt.StringValue()
		}
	}

	emojiClean := parseEmoji(emoji)
	if emojiClean == "" {
		respondEphemeral(s, i, "❌ Invalid emoji format.")
		return
	}

	// Try to find the reaction role
	reactionRole, err := m.queries.GetReactionRoleByMessageAndEmoji(ctx, dbsqlc.GetReactionRoleByMessageAndEmojiParams{
		MessageID: messageID,
		Emoji:     emojiClean,
	})
	if err != nil {
		respondEphemeral(s, i, "❌ No reaction role found for this message and emoji.")
		return
	}

	// Delete from database
	err = m.queries.DeleteReactionRoleByMessageAndEmoji(ctx, dbsqlc.DeleteReactionRoleByMessageAndEmojiParams{
		MessageID: messageID,
		Emoji:     emojiClean,
	})
	if err != nil {
		logging.Error("failed to delete reaction role", "module", "reactionroles", "error", err)
		respondEphemeral(s, i, "❌ Failed to delete reaction role from database.")
		return
	}

	// Try to remove the reaction from the message (optional, don't fail if we can't)
	_ = s.MessageReactionRemove(reactionRole.ChannelID, reactionRole.MessageID, emojiClean, "@me")

	respondEphemeral(s, i, fmt.Sprintf("✅ Reaction role with emoji %s removed from message %s.", emojiClean, messageID))
}

// handleList shows all reaction roles for the guild, sorted by channel then message.
func (m *ReactionRoles) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	reactionRoles, err := m.queries.ListReactionRolesByGuild(ctx, i.GuildID)
	if err != nil || len(reactionRoles) == 0 {
		respondEphemeral(s, i, "ℹ️ No reaction roles are configured for this server.")
		return
	}

	// Group by channel for better display
	channels := make(map[string][]dbsqlc.ReactionRole)
	for _, rr := range reactionRoles {
		channels[rr.ChannelID] = append(channels[rr.ChannelID], rr)
	}

	msg := "**Reaction Roles in this server:**\n\n"
	for channelID, rrs := range channels {
		msg += fmt.Sprintf("**Channel:** <#%s>\n", channelID)
		for _, rr := range rrs {
			msg += fmt.Sprintf("• Message `%s` - %s → <@&%s>\n", rr.MessageID, rr.Emoji, rr.RoleID)
		}
		msg += "\n"
	}

	respondEphemeral(s, i, msg)
}

// handleFix cleans up reactions on a message by removing invalid reactions and ensuring bot's reactions are present.
// It removes reactions from users who don't have the associated role and removes unauthorized emoji reactions.
func (m *ReactionRoles) handleFix(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	startTime := time.Now()
	ctx := context.Background()

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Send deferred response immediately to avoid timeout
	if err := deferResponse(s, i); err != nil {
		// If we can't send deferred response, log and return (user will see "application did not respond")
		logCommandEnd("reactionrole_fix", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "defer_response_failed",
		})
		return
	}

	var messageID string
	for _, opt := range sub.Options {
		if opt.Name == "message_id" {
			messageID = opt.StringValue()
		}
	}

	// Log fix operation start
	logDebug("fix_start", i.GuildID, userID, "starting fix operation", map[string]interface{}{
		"message_id": messageID,
	})

	// Get all reaction roles for this message
	reactionRoles, err := m.queries.ListReactionRolesByMessage(ctx, messageID)
	if err != nil || len(reactionRoles) == 0 {
		logError("fix_operation", i.GuildID, userID, "no reaction roles found for message", err, map[string]interface{}{
			"message_id": messageID,
		})
		_ = editDeferredResponse(s, i, "❌ No reaction roles found for this message.")
		logCommandEnd("reactionrole_fix", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "no_reaction_roles",
		})
		return
	}

	// Get the first reaction role to get channel ID
	channelID := reactionRoles[0].ChannelID

	// Fetch the message to verify it exists and get current reactions
	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil {
		logError("fix_operation", i.GuildID, userID, "failed to fetch message", err, map[string]interface{}{
			"message_id": messageID,
			"channel_id": channelID,
		})
		_ = editDeferredResponse(s, i, "❌ Could not find the specified message.")
		logCommandEnd("reactionrole_fix", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "message_not_found",
		})
		return
	}

	// Get all guild members to check their roles
	guildMembers, err := s.GuildMembers(i.GuildID, "", 1000)
	if err != nil {
		logError("fix_operation", i.GuildID, userID, "failed to get guild members", err, map[string]interface{}{
			"message_id": messageID,
		})
		_ = editDeferredResponse(s, i, "❌ Failed to get guild members. Please try again.")
		logCommandEnd("reactionrole_fix", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"message_id": messageID,
			"error":      "guild_members_fetch_failed",
		})
		return
	}

	// Log member count for debugging
	logDebug("fix_operation", i.GuildID, userID, "fetched guild members", map[string]interface{}{
		"member_count":           len(guildMembers),
		"reaction_role_count":    len(reactionRoles),
		"message_reaction_count": len(msg.Reactions),
	})

	// Create lookup maps for efficient checking
	// validEmojis: emoji -> role ID
	validEmojis := make(map[string]string)
	for _, rr := range reactionRoles {
		validEmojis[rr.Emoji] = rr.RoleID
	}

	// userRoles: user ID -> set of role IDs
	userRoles := make(map[string]map[string]bool)
	for _, member := range guildMembers {
		roleSet := make(map[string]bool)
		for _, roleID := range member.Roles {
			roleSet[roleID] = true
		}
		userRoles[member.User.ID] = roleSet
	}

	// Track cleanup statistics
	removedUnauthorizedEmoji := 0
	removedNoRole := 0
	keptValidReactions := 0
	totalReactionOperations := 0

	// Process each reaction on the message
	for _, reaction := range msg.Reactions {
		emoji := reaction.Emoji.APIName()
		roleID, isAuthorizedEmoji := validEmojis[emoji]

		// Get all users who reacted with this emoji
		users, err := s.MessageReactions(channelID, messageID, emoji, 100, "", "")
		if err != nil {
			logError("fix_operation", i.GuildID, userID, "failed to get users for reaction", err, map[string]interface{}{
				"emoji": emoji,
			})
			continue
		}

		// Case 1: Unauthorized emoji - remove all reactions
		if !isAuthorizedEmoji {
			for _, user := range users {
				err := s.MessageReactionRemove(channelID, messageID, emoji, user.ID)
				if err != nil {
					logError("fix_operation", i.GuildID, userID, "failed to remove unauthorized reaction", err, map[string]interface{}{
						"emoji":   emoji,
						"user_id": user.ID,
					})
				} else {
					removedUnauthorizedEmoji++
				}
				totalReactionOperations++
				// Small delay to respect rate limits
				time.Sleep(100 * time.Millisecond)
			}
			continue
		}

		// Case 2: Authorized emoji - check each user
		for _, user := range users {
			// Skip bot's own reactions
			if user.Bot {
				continue
			}

			// Check if user is still in guild and has the role
			userRoleSet, userInGuild := userRoles[user.ID]
			hasRole := userInGuild && userRoleSet[roleID]

			if !userInGuild || !hasRole {
				// User left guild or doesn't have the role - remove reaction
				err := s.MessageReactionRemove(channelID, messageID, emoji, user.ID)
				if err != nil {
					logError("fix_operation", i.GuildID, userID, "failed to remove reaction", err, map[string]interface{}{
						"emoji":   emoji,
						"user_id": user.ID,
						"role_id": roleID,
					})
				} else {
					removedNoRole++
				}
				totalReactionOperations++
				// Small delay to respect rate limits
				time.Sleep(100 * time.Millisecond)
			} else {
				// User has the role - keep reaction
				keptValidReactions++
			}
		}
	}

	// Ensure bot's reactions are present for each authorized emoji
	addedBotReactions := 0
	for emoji := range validEmojis {
		// Check if bot already reacted
		botReacted := false
		for _, reaction := range msg.Reactions {
			if reaction.Emoji.APIName() == emoji && reaction.Me {
				botReacted = true
				break
			}
		}

		if !botReacted {
			err := s.MessageReactionAdd(channelID, messageID, emoji)
			if err != nil {
				logError("fix_operation", i.GuildID, userID, "failed to add bot reaction", err, map[string]interface{}{
					"emoji": emoji,
				})
			} else {
				addedBotReactions++
			}
			totalReactionOperations++
			// Small delay to respect rate limits
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Build summary message
	msgText := fmt.Sprintf("✅ Cleaned up reactions for message `%s`.\n\n", messageID)

	if removedUnauthorizedEmoji > 0 {
		msgText += fmt.Sprintf("• Removed %d reaction(s) with unauthorized emojis\n", removedUnauthorizedEmoji)
	}

	if removedNoRole > 0 {
		msgText += fmt.Sprintf("• Removed %d reaction(s) from users without the required role\n", removedNoRole)
	}

	msgText += fmt.Sprintf("• Kept %d valid reaction(s)\n", keptValidReactions)

	if addedBotReactions > 0 {
		msgText += fmt.Sprintf("• Added %d missing bot reaction(s)\n", addedBotReactions)
	} else {
		msgText += "• All bot reactions were already present\n"
	}

	// Send final result
	if err := editDeferredResponse(s, i, msgText); err != nil {
		// Log error but command still executed successfully
		logError("fix_completion", i.GuildID, userID, "failed to send final response", err, map[string]interface{}{
			"message_id": messageID,
		})
	}

	// Log command completion with detailed statistics
	logCommandEnd("reactionrole_fix", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"message_id":             messageID,
		"removed_unauthorized":   removedUnauthorizedEmoji,
		"removed_no_role":        removedNoRole,
		"kept_valid":             keptValidReactions,
		"added_bot_reactions":    addedBotReactions,
		"total_operations":       totalReactionOperations,
		"reaction_role_count":    len(reactionRoles),
		"guild_member_count":     len(guildMembers),
		"message_reaction_count": len(msg.Reactions),
	})
}

// parseEmoji converts an emoji string to a format suitable for Discord's API.
// Supports Unicode emojis and custom emojis in various formats:
// - Unicode: ✅, 🔥, 🎉
// - Custom: :emoji_name:, :emoji_name:id, <:emoji_name:id>, <a:emoji_name:id>
func parseEmoji(input string) string {
	input = strings.TrimSpace(input)

	if input == "" {
		return ""
	}

	// Handle Discord's full custom emoji format with angle brackets
	// Examples: <:custom_emoji:1234567890>, <a:animated_emoji:1234567890>
	if strings.HasPrefix(input, "<") && strings.HasSuffix(input, ">") {
		// Remove angle brackets
		input = strings.TrimPrefix(input, "<")
		input = strings.TrimSuffix(input, ">")

		// Handle animated emoji prefix
		if strings.HasPrefix(input, "a:") {
			input = strings.TrimPrefix(input, "a:")
		}

		// Now input should be in format :name:id or name:id
		// Remove leading colon if present
		if strings.HasPrefix(input, ":") {
			input = strings.TrimPrefix(input, ":")
		}

		// Split by : to get name and id
		parts := strings.Split(input, ":")
		if len(parts) == 2 {
			// Clean up the ID - remove any trailing characters
			id := strings.TrimSpace(parts[1])
			// Return in name:id format
			return parts[0] + ":" + id
		} else if len(parts) == 1 {
			// Just name, return as-is
			return parts[0]
		}
	}

	// Handle traditional custom emoji format without angle brackets
	// Examples: :emoji_name:, :emoji_name:id
	if strings.HasPrefix(input, ":") {
		// Remove the leading colon
		clean := strings.TrimPrefix(input, ":")
		// Split by : to separate name and id
		parts := strings.Split(clean, ":")
		if len(parts) == 2 {
			// Handle case where second part might be empty (format :name:)
			if parts[1] == "" {
				return parts[0]
			}
			// Custom emoji with ID: format is name:id
			return parts[0] + ":" + parts[1]
		} else if len(parts) == 1 {
			// Check if it ends with : (format :name:)
			if strings.HasSuffix(clean, ":") {
				return strings.TrimSuffix(clean, ":")
			}
			// Just name
			return clean
		}
	}

	// Assume it's a Unicode emoji or already in correct format (name:id)
	return input
}
