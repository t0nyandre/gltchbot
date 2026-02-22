package autorole

import (
	"context"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// handleGuildMemberAdd is called when a user joins a guild.
func (m *AutoRole) handleGuildMemberAdd(s *discordgo.Session, gm *discordgo.GuildMemberAdd) {
	// Ignore bots
	if gm.User.Bot {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: gm.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Check if user has already triggered 'join' for this guild
	_, err = m.queries.GetUserTrigger(ctx, dbsqlc.GetUserTriggerParams{
		GuildID: gm.GuildID,
		UserID:  gm.User.ID,
		Trigger: "join",
	})
	if err == nil {
		// User already triggered join before (re-join?), do nothing
		return
	}

	// Get all auto roles for this guild with trigger 'join'
	autoRoles, err := m.queries.ListAutoRolesByGuildAndTrigger(ctx, dbsqlc.ListAutoRolesByGuildAndTriggerParams{
		GuildID: gm.GuildID,
		Trigger: "join",
	})
	if err != nil {
		logError("guild_member_add", gm.GuildID, gm.User.ID, "failed to list auto roles", err, nil)
		return
	}

	if len(autoRoles) == 0 {
		// No join auto roles configured
		return
	}

	// Assign each role
	for _, autoRole := range autoRoles {
		err := s.GuildMemberRoleAdd(gm.GuildID, gm.User.ID, autoRole.RoleID)
		if err != nil {
			logError("guild_member_add", gm.GuildID, gm.User.ID, "failed to assign role", err, map[string]interface{}{
				"role_id": autoRole.RoleID,
			})
			// Continue with other roles
		}
	}

	// Record the trigger
	err = m.queries.CreateUserTrigger(ctx, dbsqlc.CreateUserTriggerParams{
		GuildID: gm.GuildID,
		UserID:  gm.User.ID,
		Trigger: "join",
	})
	if err != nil {
		logError("guild_member_add", gm.GuildID, gm.User.ID, "failed to record user trigger", err, nil)
	}
}

// handleMessageCreate is called when a message is sent in a guild.
func (m *AutoRole) handleMessageCreate(s *discordgo.Session, mc *discordgo.MessageCreate) {
	// Ignore bots and DMs
	if mc.Author.Bot || mc.GuildID == "" {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: mc.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Check if user has already triggered 'first_message' for this guild
	_, err = m.queries.GetUserTrigger(ctx, dbsqlc.GetUserTriggerParams{
		GuildID: mc.GuildID,
		UserID:  mc.Author.ID,
		Trigger: "first_message",
	})
	if err == nil {
		// Already triggered
		return
	}

	// Get all auto roles for this guild with trigger 'first_message'
	autoRoles, err := m.queries.ListAutoRolesByGuildAndTrigger(ctx, dbsqlc.ListAutoRolesByGuildAndTriggerParams{
		GuildID: mc.GuildID,
		Trigger: "first_message",
	})
	if err != nil {
		logError("message_create", mc.GuildID, mc.Author.ID, "failed to list auto roles", err, nil)
		return
	}

	if len(autoRoles) == 0 {
		// No first_message auto roles configured
		return
	}

	// Assign each role
	for _, autoRole := range autoRoles {
		err := s.GuildMemberRoleAdd(mc.GuildID, mc.Author.ID, autoRole.RoleID)
		if err != nil {
			logError("message_create", mc.GuildID, mc.Author.ID, "failed to assign role", err, map[string]interface{}{
				"role_id": autoRole.RoleID,
			})
			// Continue with other roles
		}
	}

	// Record the trigger
	err = m.queries.CreateUserTrigger(ctx, dbsqlc.CreateUserTriggerParams{
		GuildID: mc.GuildID,
		UserID:  mc.Author.ID,
		Trigger: "first_message",
	})
	if err != nil {
		logError("message_create", mc.GuildID, mc.Author.ID, "failed to record user trigger", err, nil)
	}
}

// handleMessageReactionAdd is called when a user adds a reaction to a message.
func (m *AutoRole) handleMessageReactionAdd(s *discordgo.Session, mr *discordgo.MessageReactionAdd) {
	// Ignore DMs
	if mr.GuildID == "" {
		return
	}

	// Get member to check if bot (we'll fetch member if needed)
	// Actually we need to check if the reacting user is a bot. We'll fetch member.
	// For simplicity, we can ignore if the user is a bot, but we need to get the user ID.
	// We'll rely on the user ID from mr.UserID.
	// We'll need to fetch member to check bot flag. We'll use session.GuildMember.
	// However, that's an API call. We'll assume bots are not reacting, but we can skip.
	// Let's fetch guild member; if error, just return.
	member, err := s.GuildMember(mr.GuildID, mr.UserID)
	if err != nil {
		return
	}
	if member.User.Bot {
		return
	}

	ctx := context.Background()

	// Check if module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: mr.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Check if user has already triggered 'first_reaction' for this guild
	_, err = m.queries.GetUserTrigger(ctx, dbsqlc.GetUserTriggerParams{
		GuildID: mr.GuildID,
		UserID:  mr.UserID,
		Trigger: "first_reaction",
	})
	if err == nil {
		// Already triggered
		return
	}

	// Get all auto roles for this guild with trigger 'first_reaction'
	autoRoles, err := m.queries.ListAutoRolesByGuildAndTrigger(ctx, dbsqlc.ListAutoRolesByGuildAndTriggerParams{
		GuildID: mr.GuildID,
		Trigger: "first_reaction",
	})
	if err != nil {
		logError("message_reaction_add", mr.GuildID, mr.UserID, "failed to list auto roles", err, nil)
		return
	}

	if len(autoRoles) == 0 {
		// No first_reaction auto roles configured
		return
	}

	// Assign each role
	for _, autoRole := range autoRoles {
		err := s.GuildMemberRoleAdd(mr.GuildID, mr.UserID, autoRole.RoleID)
		if err != nil {
			logError("message_reaction_add", mr.GuildID, mr.UserID, "failed to assign role", err, map[string]interface{}{
				"role_id": autoRole.RoleID,
			})
			// Continue with other roles
		}
	}

	// Record the trigger
	err = m.queries.CreateUserTrigger(ctx, dbsqlc.CreateUserTriggerParams{
		GuildID: mr.GuildID,
		UserID:  mr.UserID,
		Trigger: "first_reaction",
	})
	if err != nil {
		logError("message_reaction_add", mr.GuildID, mr.UserID, "failed to record user trigger", err, nil)
	}
}
