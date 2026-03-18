package ratelimit

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// RateLimitedSession wraps a discordgo.Session and applies rate limiting to API calls.
type RateLimitedSession struct {
	*discordgo.Session
	manager *Manager
}

// NewRateLimitedSession creates a new rate-limited session wrapper.
func NewRateLimitedSession(s *discordgo.Session) *RateLimitedSession {
	return &RateLimitedSession{
		Session: s,
		manager: NewManager(),
	}
}

// wait calls Manager.Wait with a background context and the given endpoint.
func (r *RateLimitedSession) wait(endpoint string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = r.manager.Wait(ctx, endpoint) // ignore error (timeout or cancel)
}

// ChannelCreateComplex wraps discordgo.Session.ChannelCreateComplex with rate limiting.
func (r *RateLimitedSession) ChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	r.wait(EndpointChannelCreate)
	return r.Session.ChannelCreateComplex(guildID, data)
}

// GuildChannelCreateComplex wraps discordgo.Session.GuildChannelCreateComplex with rate limiting.
// This is an alias for ChannelCreateComplex, but we still wrap it for consistency.
func (r *RateLimitedSession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	r.wait(EndpointChannelCreate)
	return r.Session.GuildChannelCreateComplex(guildID, data)
}

// ChannelDelete wraps discordgo.Session.ChannelDelete with rate limiting.
func (r *RateLimitedSession) ChannelDelete(channelID string) (*discordgo.Channel, error) {
	r.wait(EndpointChannelDelete)
	return r.Session.ChannelDelete(channelID)
}

// GuildMemberMove wraps discordgo.Session.GuildMemberMove with rate limiting.
func (r *RateLimitedSession) GuildMemberMove(guildID, userID string, channelID *string) error {
	r.wait(EndpointGuildMemberMove)
	return r.Session.GuildMemberMove(guildID, userID, channelID)
}

// Channel wraps discordgo.Session.Channel with rate limiting.
func (r *RateLimitedSession) Channel(channelID string) (*discordgo.Channel, error) {
	r.wait(EndpointChannel)
	return r.Session.Channel(channelID)
}

// GuildMember wraps discordgo.Session.GuildMember with rate limiting.
func (r *RateLimitedSession) GuildMember(guildID, userID string) (*discordgo.Member, error) {
	r.wait(EndpointGuildMember)
	return r.Session.GuildMember(guildID, userID)
}

// Guild wraps discordgo.Session.Guild with rate limiting.
func (r *RateLimitedSession) Guild(guildID string) (*discordgo.Guild, error) {
	r.wait(EndpointGuild)
	return r.Session.Guild(guildID)
}

// InteractionRespond wraps discordgo.Session.InteractionRespond with rate limiting.
func (r *RateLimitedSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	r.wait(EndpointInteractionResponse)
	return r.Session.InteractionRespond(interaction, resp)
}

// InteractionResponseEdit wraps discordgo.Session.InteractionResponseEdit with rate limiting.
func (r *RateLimitedSession) InteractionResponseEdit(interaction *discordgo.Interaction, data *discordgo.WebhookEdit) (*discordgo.Message, error) {
	r.wait(EndpointInteractionResponse)
	return r.Session.InteractionResponseEdit(interaction, data)
}

// GuildMemberRoleAdd wraps discordgo.Session.GuildMemberRoleAdd with rate limiting.
func (r *RateLimitedSession) GuildMemberRoleAdd(guildID, userID, roleID string) error {
	r.wait(EndpointGuildMemberRoleAdd)
	return r.Session.GuildMemberRoleAdd(guildID, userID, roleID)
}

// MessageReactionAdd wraps discordgo.Session.MessageReactionAdd with rate limiting.
func (r *RateLimitedSession) MessageReactionAdd(channelID, messageID, emojiID string) error {
	r.wait(EndpointMessageReactionAdd)
	return r.Session.MessageReactionAdd(channelID, messageID, emojiID)
}

// MessageReactionRemove wraps discordgo.Session.MessageReactionRemove with rate limiting.
func (r *RateLimitedSession) MessageReactionRemove(channelID, messageID, emojiID string) error {
	r.wait(EndpointMessageReactionRemove)
	return r.Session.MessageReactionRemove(channelID, messageID, emojiID)
}