package jointocreate

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// handleVoiceStateUpdate fires whenever a user joins, leaves, or moves between voice channels.
func (m *JoinToCreate) handleVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	ctx := context.Background()

	// --- User left a channel (or moved away from one) ---
	if vs.BeforeUpdate != nil && vs.BeforeUpdate.ChannelID != "" {
		m.maybeCleanupChannel(ctx, s, vs.BeforeUpdate.ChannelID)
	}

	// --- User joined a channel ---
	if vs.ChannelID == "" {
		return // user disconnected entirely, nothing to create
	}

	// Check if the joined channel is a JTC parent channel
	parent, err := m.queries.GetJTCParentChannel(ctx, vs.ChannelID)
	if err != nil {
		if err != pgx.ErrNoRows {
			log.Printf("[jointocreate] error checking parent channel %s: %v", vs.ChannelID, err)
		}
		return // not a parent channel, nothing to do
	}

	// Check that the JTC module is enabled for this guild
	enabled, err := m.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: vs.GuildID,
		Name:    moduleName,
	})
	if err != nil || !enabled {
		return
	}

	// Resolve the channel name to use for the new temp channel
	channelName := m.resolveChannelName(ctx, s, vs.GuildID, vs.UserID)

	// Fetch the parent channel to copy its settings (bitrate, user limit, etc.)
	parentCh, err := s.Channel(parent.ChannelID)
	if err != nil {
		log.Printf("[jointocreate] failed to fetch parent channel %s: %v", parent.ChannelID, err)
		return
	}

	// Clone the parent channel with the user's name
	newCh, err := s.GuildChannelCreateComplex(vs.GuildID, discordgo.GuildChannelCreateData{
		Name:      channelName,
		Type:      discordgo.ChannelTypeGuildVoice,
		ParentID:  parent.CategoryID,
		Bitrate:   parentCh.Bitrate,
		UserLimit: parentCh.UserLimit,
	})
	if err != nil {
		log.Printf("[jointocreate] failed to create temp channel: %v", err)
		return
	}

	// Save the new channel to the database
	if _, err := m.queries.CreateJTCActiveChannel(ctx, dbsqlc.CreateJTCActiveChannelParams{
		ChannelID: newCh.ID,
		GuildID:   vs.GuildID,
		OwnerID:   vs.UserID,
		ParentID:  parent.ChannelID,
	}); err != nil {
		log.Printf("[jointocreate] failed to save active channel: %v", err)
		_, _ = s.ChannelDelete(newCh.ID)
		return
	}

	// Move the user into the new channel
	if err := s.GuildMemberMove(vs.GuildID, vs.UserID, &newCh.ID); err != nil {
		log.Printf("[jointocreate] failed to move user %s to channel %s: %v", vs.UserID, newCh.ID, err)
	}
}

// handleChannelUpdate fires when a channel's settings change (e.g. name rename).
// We use this to persist the user's custom channel name.
func (m *JoinToCreate) handleChannelUpdate(s *discordgo.Session, cu *discordgo.ChannelUpdate) {
	ctx := context.Background()

	// Check if this is an active JTC channel
	active, err := m.queries.GetJTCActiveChannel(ctx, cu.ID)
	if err != nil {
		return // not a JTC channel
	}

	// Save the new name as the user's preference
	if err := m.queries.UpsertJTCUserSettings(ctx, dbsqlc.UpsertJTCUserSettingsParams{
		GuildID:    active.GuildID,
		UserID:     active.OwnerID,
		CustomName: pgtype.Text{String: cu.Name, Valid: true},
	}); err != nil {
		log.Printf("[jointocreate] failed to save user channel name preference: %v", err)
	}
}

// handleChannelDelete fires when any channel is deleted (e.g. manually by an admin).
// We clean up our DB record so we don't have stale entries.
func (m *JoinToCreate) handleChannelDelete(s *discordgo.Session, cd *discordgo.ChannelDelete) {
	ctx := context.Background()

	// Silently delete from active channels — if it's not there, that's fine
	if err := m.queries.DeleteJTCActiveChannel(ctx, cd.ID); err != nil {
		if err != pgx.ErrNoRows {
			log.Printf("[jointocreate] error cleaning up deleted channel %s: %v", cd.ID, err)
		}
	}
}

// countUsersInChannel returns the number of users currently in a voice channel.
// It tries to use DiscordGo's state cache first, then falls back to fetching guild data.
func (m *JoinToCreate) countUsersInChannel(s *discordgo.Session, guildID, channelID string) (int, error) {
	// Try to get guild from state cache first
	if guild, err := s.State.Guild(guildID); err == nil && guild != nil {
		count := 0
		for _, vs := range guild.VoiceStates {
			if vs.ChannelID == channelID {
				count++
			}
		}
		return count, nil
	}

	// Fallback: fetch the guild via API to get voice states
	guild, err := s.Guild(guildID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			count++
		}
	}
	return count, nil
}

// maybeCleanupChannel deletes a temp channel from Discord and DB if it's empty.
func (m *JoinToCreate) maybeCleanupChannel(ctx context.Context, s *discordgo.Session, channelID string) {
	// Is it an active JTC channel?
	active, err := m.queries.GetJTCActiveChannel(ctx, channelID)
	if err != nil {
		return // not a JTC active channel
	}

	// Count users in the channel using voice states (more reliable than ch.Members)
	userCount, err := m.countUsersInChannel(s, active.GuildID, channelID)
	if err != nil {
		// If we can't get voice states, fall back to the old method
		ch, err := s.Channel(channelID)
		if err != nil {
			// Channel might already be gone — clean up DB
			_ = m.queries.DeleteJTCActiveChannel(ctx, channelID)
			return
		}
		userCount = len(ch.Members)
	}

	// Only delete if the channel is truly empty
	if userCount > 0 {
		return
	}

	// Delete from Discord
	if _, err := s.ChannelDelete(channelID); err != nil {
		log.Printf("[jointocreate] failed to delete empty channel %s: %v", channelID, err)
	}

	// Remove from DB
	if err := m.queries.DeleteJTCActiveChannel(ctx, channelID); err != nil {
		log.Printf("[jointocreate] failed to remove active channel %s from db: %v", channelID, err)
	}
}

// resolveChannelName determines the name for a new temp channel.
// Priority: saved custom name > server nickname > global display name > username.
func (m *JoinToCreate) resolveChannelName(ctx context.Context, s *discordgo.Session, guildID, userID string) string {
	// Check for a saved preference first
	settings, err := m.queries.GetJTCUserSettings(ctx, dbsqlc.GetJTCUserSettingsParams{
		GuildID: guildID,
		UserID:  userID,
	})
	if err == nil && settings.CustomName.Valid && settings.CustomName.String != "" {
		return settings.CustomName.String
	}

	// Fall back to Discord member info
	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		return "Temporary Channel"
	}

	var displayName string
	if member.Nick != "" {
		// Server nickname takes priority
		displayName = member.Nick
	} else if member.User.GlobalName != "" {
		// Global display name second
		displayName = member.User.GlobalName
	} else {
		// Username as final fallback
		displayName = member.User.Username
	}

	return fmt.Sprintf("%s's Channel", displayName)
}
