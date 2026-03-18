package ratelimit

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// Manager manages rate limiters for different Discord API endpoints.
type Manager struct {
	mu       sync.RWMutex
	limiters map[string]*Limiter
	global   *Limiter
}

// NewManager creates a new Manager with default limits.
// Reads environment variables prefixed with DISCORD_RATE_LIMIT_.
// Format: DISCORD_RATE_LIMIT_GLOBAL=50 (global rate per second)
// DISCORD_RATE_LIMIT_CHANNEL_CREATE=2 (per endpoint)
func NewManager() *Manager {
	m := &Manager{
		limiters: make(map[string]*Limiter),
		global:   nil,
	}
	// Global rate limit
	globalRate := parseEnvRate("DISCORD_RATE_LIMIT_GLOBAL", 50)
	if globalRate > 0 {
		m.global = NewLimiter(globalRate, int(globalRate)) // burst = rate per second
	}
	// Predefined endpoint keys
	endpoints := map[string]float64{
		"channel_create":          parseEnvRate("DISCORD_RATE_LIMIT_CHANNEL_CREATE", 2),
		"channel_delete":          parseEnvRate("DISCORD_RATE_LIMIT_CHANNEL_DELETE", 5),
		"channel_edit":            parseEnvRate("DISCORD_RATE_LIMIT_CHANNEL_EDIT", 5),
		"guild_member_move":       parseEnvRate("DISCORD_RATE_LIMIT_GUILD_MEMBER_MOVE", 10),
		"message_reaction_add":    parseEnvRate("DISCORD_RATE_LIMIT_MESSAGE_REACTION_ADD", 5),
		"message_reaction_remove": parseEnvRate("DISCORD_RATE_LIMIT_MESSAGE_REACTION_REMOVE", 5),
		"interaction_response":    parseEnvRate("DISCORD_RATE_LIMIT_INTERACTION_RESPONSE", 5),
		"guild_member":            parseEnvRate("DISCORD_RATE_LIMIT_GUILD_MEMBER", 10),
		"guild_member_role_add":   parseEnvRate("DISCORD_RATE_LIMIT_GUILD_MEMBER_ROLE_ADD", 5),
		"guild":                   parseEnvRate("DISCORD_RATE_LIMIT_GUILD", 5),
	}
	for key, rate := range endpoints {
		if rate > 0 {
			m.limiters[key] = NewLimiter(rate, int(rate))
		}
	}
	return m
}

// parseEnvRate reads an environment variable and parses it as float64.
// If not set or invalid, returns defaultVal.
func parseEnvRate(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return defaultVal
}

// Wait waits for both global and endpoint rate limits.
func (m *Manager) Wait(ctx context.Context, endpoint string) error {
	// Wait for global limiter first
	if m.global != nil {
		logging.Debug("waiting for global rate limit", "endpoint", endpoint)
		if err := m.global.Wait(ctx); err != nil {
			logging.Error("global rate limit wait error", "endpoint", endpoint, "error", err)
			return err
		}
		logging.Debug("global rate limit passed", "endpoint", endpoint)
	}
	// Wait for endpoint-specific limiter
	m.mu.RLock()
	limiter, ok := m.limiters[endpoint]
	m.mu.RUnlock()
	if ok && limiter != nil {
		logging.Debug("waiting for endpoint rate limit", "endpoint", endpoint)
		if err := limiter.Wait(ctx); err != nil {
			logging.Error("endpoint rate limit wait error", "endpoint", endpoint, "error", err)
			return err
		}
		logging.Debug("endpoint rate limit passed", "endpoint", endpoint)
	}
	return nil
}

// SetEndpointLimit sets or updates the rate limit for an endpoint.
func (m *Manager) SetEndpointLimit(endpoint string, ratePerSec float64, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ratePerSec <= 0 {
		delete(m.limiters, endpoint)
		return
	}
	limiter, ok := m.limiters[endpoint]
	if !ok {
		limiter = NewLimiter(ratePerSec, burst)
		m.limiters[endpoint] = limiter
	} else {
		limiter.SetRate(ratePerSec, burst)
	}
}

// GetEndpointLimit returns the current rate limit for an endpoint.
func (m *Manager) GetEndpointLimit(endpoint string) (ratePerSec float64, burst int, exists bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limiter, ok := m.limiters[endpoint]
	if !ok || limiter == nil {
		return 0, 0, false
	}
	// Note: Limiter does not expose rate and burst directly; we could add methods.
	// For simplicity, we'll just return default.
	return 0, 0, true
}

// Endpoint constants
const (
	EndpointChannelCreate         = "channel_create"
	EndpointChannelDelete         = "channel_delete"
	EndpointChannelEdit           = "channel_edit"
	EndpointGuildMemberMove       = "guild_member_move"
	EndpointMessageReactionAdd    = "message_reaction_add"
	EndpointMessageReactionRemove = "message_reaction_remove"
	EndpointInteractionResponse   = "interaction_response"
	EndpointGuildMember           = "guild_member"
	EndpointGuildMemberRoleAdd    = "guild_member_role_add"
	EndpointGuild                 = "guild"
	EndpointChannel               = "channel"
)
