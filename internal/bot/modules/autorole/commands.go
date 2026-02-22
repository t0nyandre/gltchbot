package autorole

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// logStructured logs a structured message with consistent fields
func logStructured(level, module, operation, guildID, userID, message string, data map[string]interface{}) {
	// Build structured log message
	logMsg := fmt.Sprintf("[%s] module=%s operation=%s", level, module, operation)

	if guildID != "" {
		logMsg += fmt.Sprintf(" guild_id=%s", guildID)
	}
	if userID != "" {
		logMsg += fmt.Sprintf(" user_id=%s", userID)
	}
	if message != "" {
		logMsg += fmt.Sprintf(" message=%q", message)
	}

	// Add data fields
	for key, value := range data {
		logMsg += fmt.Sprintf(" %s=%v", key, value)
	}

	log.Println(logMsg)
}

// logCommandStart logs the start of a command execution
func logCommandStart(command, guildID, userID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["command"] = command
	logStructured("INFO", "autorole", "command_start", guildID, userID, "", data)
}

// logCommandEnd logs the end of a command execution with duration
func logCommandEnd(command, guildID, userID string, duration time.Duration, success bool, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["command"] = command
	data["duration_ms"] = duration.Milliseconds()
	data["success"] = success
	logStructured("INFO", "autorole", "command_end", guildID, userID, "", data)
}

// logError logs an error with structured fields
func logError(operation, guildID, userID, message string, err error, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["error"] = err.Error()
	logStructured("ERROR", "autorole", operation, guildID, userID, message, data)
}

// logDebug logs a debug message with structured fields
func logDebug(operation, guildID, userID, message string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	logStructured("DEBUG", "autorole", operation, guildID, userID, message, data)
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

// handleInteraction is the main entry point for slash command interactions.
func (m *AutoRole) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	startTime := time.Now()

	// Only handle slash commands
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Only handle our command
	if i.ApplicationCommandData().Name != "autorole" {
		return
	}

	// Get user ID for logging
	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Log command start
	logCommandStart("autorole", i.GuildID, userID, map[string]interface{}{
		"subcommand": i.ApplicationCommandData().Options[0].Name,
	})

	// Require administrator permission
	if i.Member == nil || i.Member.Permissions&discordgo.PermissionAdministrator == 0 {
		respondEphemeral(s, i, "❌ You need the **Administrator** permission to use this command.")
		logCommandEnd("autorole", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
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
		respondEphemeral(s, i, "❌ The AutoRole module is not enabled for this server. Please enable it first via the dashboard.")
		logCommandEnd("autorole", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
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
	}

	// Log successful command routing
	logCommandEnd("autorole", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"subcommand": sub.Name,
	})
}

// handleAdd creates a new auto role entry.
func (m *AutoRole) handleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	startTime := time.Now()
	ctx := context.Background()

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Send deferred response immediately to avoid timeout
	if err := deferResponse(s, i); err != nil {
		// If we can't send deferred response, log and return (user will see "application did not respond")
		logCommandEnd("autorole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "defer_response_failed",
		})
		return
	}

	var roleID, trigger string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "role":
			roleID = opt.Value.(string)
		case "trigger":
			trigger = opt.Value.(string)
		}
	}

	// Check if auto role already exists for this guild+role+trigger
	_, err := m.queries.GetAutoRoleByGuildAndRoleAndTrigger(ctx, dbsqlc.GetAutoRoleByGuildAndRoleAndTriggerParams{
		GuildID: i.GuildID,
		RoleID:  roleID,
		Trigger: trigger,
	})
	if err == nil {
		// Auto role already exists
		editDeferredResponse(s, i, fmt.Sprintf("❌ An auto role for <@&%s> with trigger `%s` already exists.", roleID, trigger))
		logCommandEnd("autorole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"role_id": roleID,
			"trigger": trigger,
			"error":   "already_exists",
		})
		return
	}

	// Create the auto role
	autoRole, err := m.queries.CreateAutoRole(ctx, dbsqlc.CreateAutoRoleParams{
		GuildID: i.GuildID,
		RoleID:  roleID,
		Trigger: trigger,
	})
	if err != nil {
		editDeferredResponse(s, i, fmt.Sprintf("❌ Failed to create auto role: %v", err))
		logCommandEnd("autorole_add", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"role_id": roleID,
			"trigger": trigger,
			"error":   err.Error(),
		})
		return
	}

	editDeferredResponse(s, i, fmt.Sprintf("✅ Auto role created! <@&%s> will be automatically assigned to users when they **%s**.", roleID, getTriggerDescription(trigger)))
	logCommandEnd("autorole_add", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"role_id":      roleID,
		"trigger":      trigger,
		"auto_role_id": autoRole.ID,
	})
}

// handleRemove deletes an auto role entry.
func (m *AutoRole) handleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	startTime := time.Now()
	ctx := context.Background()

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Send deferred response immediately to avoid timeout
	if err := deferResponse(s, i); err != nil {
		logCommandEnd("autorole_remove", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "defer_response_failed",
		})
		return
	}

	var roleID, trigger string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "role":
			roleID = opt.Value.(string)
		case "trigger":
			trigger = opt.Value.(string)
		}
	}

	// Delete the auto role
	err := m.queries.DeleteAutoRoleByGuildAndRoleAndTrigger(ctx, dbsqlc.DeleteAutoRoleByGuildAndRoleAndTriggerParams{
		GuildID: i.GuildID,
		RoleID:  roleID,
		Trigger: trigger,
	})
	if err != nil {
		editDeferredResponse(s, i, fmt.Sprintf("❌ Failed to remove auto role: %v", err))
		logCommandEnd("autorole_remove", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"role_id": roleID,
			"trigger": trigger,
			"error":   err.Error(),
		})
		return
	}

	editDeferredResponse(s, i, fmt.Sprintf("✅ Auto role removed! <@&%s> will no longer be automatically assigned for trigger `%s`.", roleID, trigger))
	logCommandEnd("autorole_remove", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"role_id": roleID,
		"trigger": trigger,
	})
}

// handleList shows all configured auto roles for the guild.
func (m *AutoRole) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	startTime := time.Now()
	ctx := context.Background()

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}

	// Send deferred response immediately to avoid timeout
	if err := deferResponse(s, i); err != nil {
		logCommandEnd("autorole_list", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": "defer_response_failed",
		})
		return
	}

	// Get all auto roles for this guild
	autoRoles, err := m.queries.ListAutoRolesByGuild(ctx, i.GuildID)
	if err != nil {
		editDeferredResponse(s, i, fmt.Sprintf("❌ Failed to list auto roles: %v", err))
		logCommandEnd("autorole_list", i.GuildID, userID, time.Since(startTime), false, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if len(autoRoles) == 0 {
		editDeferredResponse(s, i, "📭 No auto roles configured for this server.")
		logCommandEnd("autorole_list", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
			"count": 0,
		})
		return
	}

	// Group by trigger for better presentation
	byTrigger := make(map[string][]string)
	for _, ar := range autoRoles {
		desc := fmt.Sprintf("<@&%s>", ar.RoleID)
		byTrigger[ar.Trigger] = append(byTrigger[ar.Trigger], desc)
	}

	var sb strings.Builder
	sb.WriteString("## Auto Roles for this server\n\n")

	// Show join triggers first, then first_message, then first_reaction
	triggers := []string{"join", "first_message", "first_reaction"}
	for _, trigger := range triggers {
		roles, ok := byTrigger[trigger]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("**%s**:\n", getTriggerDescription(trigger)))
		for _, role := range roles {
			sb.WriteString(fmt.Sprintf("• %s\n", role))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Total: %d auto role(s)", len(autoRoles)))

	editDeferredResponse(s, i, sb.String())
	logCommandEnd("autorole_list", i.GuildID, userID, time.Since(startTime), true, map[string]interface{}{
		"count": len(autoRoles),
	})
}

// getTriggerDescription returns a human-readable description of a trigger.
func getTriggerDescription(trigger string) string {
	switch trigger {
	case "join":
		return "join the server"
	case "first_message":
		return "send their first message"
	case "first_reaction":
		return "add their first reaction"
	default:
		return trigger
	}
}
