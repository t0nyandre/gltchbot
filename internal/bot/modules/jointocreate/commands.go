package jointocreate

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// handleInteraction routes slash command interactions for this module.
func (m *JoinToCreate) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "jointocreate" {
		return
	}

	// Must be used in a guild
	if i.GuildID == "" {
		respondEphemeral(s, i, "This command can only be used in a server.")
		return
	}

	// Require administrator permission
	if i.Member == nil || i.Member.Permissions&discordgo.PermissionAdministrator == 0 {
		respondEphemeral(s, i, "❌ You need the **Administrator** permission to use this command.")
		return
	}

	sub := i.ApplicationCommandData().Options[0]
	switch sub.Name {
	case "setup":
		m.handleSetup(s, i, sub)
	case "remove":
		m.handleRemove(s, i, sub)
	case "list":
		m.handleList(s, i)
	}
}

// handleSetup creates a new JoinToCreate parent voice channel in the specified category.
func (m *JoinToCreate) handleSetup(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	ctx := context.Background()

	var categoryID, channelName string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "category":
			categoryID = opt.ChannelValue(s).ID
		case "channel_name":
			channelName = opt.StringValue()
		}
	}

	// Verify the category exists and is actually a category
	category, err := s.Channel(categoryID)
	if err != nil || category.Type != discordgo.ChannelTypeGuildCategory {
		respondEphemeral(s, i, "❌ Invalid category selected.")
		return
	}

	// Check module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: i.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		respondEphemeral(s, i, "❌ The JoinToCreate module is not enabled for this server. Please enable it first via the dashboard.")
		return
	}

	// Create the voice channel in Discord
	ch, err := s.GuildChannelCreateComplex(i.GuildID, discordgo.GuildChannelCreateData{
		Name:     channelName,
		Type:     discordgo.ChannelTypeGuildVoice,
		ParentID: categoryID,
	})
	if err != nil {
		logging.Error("failed to create channel", "module", "jointocreate", "error", err)
		respondEphemeral(s, i, "❌ Failed to create the voice channel. Make sure I have the **Manage Channels** permission.")
		return
	}

	// Save to database
	parent, err := m.queries.CreateJTCParentChannel(ctx, dbsqlc.CreateJTCParentChannelParams{
		GuildID:     i.GuildID,
		ChannelID:   ch.ID,
		CategoryID:  categoryID,
		ChannelName: channelName,
	})
	if err != nil {
		logging.Error("failed to save parent channel", "module", "jointocreate", "error", err)
		// Clean up the Discord channel we just created
		_, _ = s.ChannelDelete(ch.ID)
		respondEphemeral(s, i, "❌ Failed to save the channel configuration to the database.")
		return
	}
	// Add to cache
	m.cache.Set(parent.ChannelID, &parent)

	respondEphemeral(s, i, fmt.Sprintf("✅ JoinToCreate channel **%s** has been set up in **%s**! Users who join it will get their own temporary channel.", channelName, category.Name))
}

// handleRemove deletes a JoinToCreate parent channel from Discord and the database.
func (m *JoinToCreate) handleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	ctx := context.Background()

	channelID := sub.Options[0].ChannelValue(s).ID

	// Check it's actually a JTC parent channel
	parent := m.getParentChannel(ctx, channelID)
	if parent == nil {
		respondEphemeral(s, i, "❌ That channel is not a JoinToCreate parent channel.")
		return
	}

	// Delete from database (cascade will clean up active channels too)
	if err := m.queries.DeleteJTCParentChannel(ctx, dbsqlc.DeleteJTCParentChannelParams{
		ChannelID: channelID,
		GuildID:   i.GuildID,
	}); err != nil {
		respondEphemeral(s, i, "❌ Failed to remove the channel from the database.")
		return
	}
	// Invalidate cache
	m.invalidateParentChannel(channelID)

	// Delete the Discord channel
	if _, err := s.ChannelDelete(channelID); err != nil {
		logging.Error("failed to delete Discord channel", "module", "jointocreate", "channel_id", channelID, "error", err)
	}

	respondEphemeral(s, i, "✅ JoinToCreate parent channel removed successfully.")
}

// handleList shows all JoinToCreate parent channels for this guild.
func (m *JoinToCreate) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	parents, err := m.queries.ListJTCParentChannels(ctx, i.GuildID)
	if err != nil || len(parents) == 0 {
		respondEphemeral(s, i, "ℹ️ No JoinToCreate channels are configured for this server.")
		return
	}

	msg := "**JoinToCreate parent channels:**\n"
	for _, p := range parents {
		msg += fmt.Sprintf("• <#%s> — `%s`\n", p.ChannelID, p.ChannelName)
	}

	respondEphemeral(s, i, msg)
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
