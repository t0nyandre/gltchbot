package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
	"github.com/t0nyandre/gltchbot/internal/bot/ratelimit"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// RateLimit returns a middleware that applies IP-based rate limiting with token bucket algorithm.
// Parameters:
//   - globalRate: global requests per second across all IPs (0 = no global limit)
//   - authRate: requests per second per IP for authenticated endpoints
//   - unauthRate: requests per second per IP for unauthenticated endpoints
//   - burstMultiplier: multiplier for burst size (burst = rate * multiplier)
//
// The middleware extracts client IP from X-Forwarded-For header (if behind proxy) or RemoteAddr.
// Authenticated requests are identified by the presence of X-API-Key header.
// The health endpoint (/health) is always considered unauthenticated.
func RateLimit(globalRate, authRate, unauthRate, burstMultiplier float64) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		globalRate:      globalRate,
		authRate:        authRate,
		unauthRate:      unauthRate,
		burstMultiplier: burstMultiplier,
		ipLimiters:      sync.Map{},
		stop:            make(chan struct{}),
	}
	// Create global limiter if rate > 0
	if globalRate > 0 {
		burst := int(globalRate * burstMultiplier)
		if burst < 1 {
			burst = 1
		}
		rl.global = ratelimit.NewLimiter(globalRate, burst)
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rl.serveHTTP(w, r, next)
		})
	}
}

type rateLimiter struct {
	global          *ratelimit.Limiter
	globalRate      float64
	authRate        float64
	unauthRate      float64
	burstMultiplier float64
	ipLimiters      sync.Map // map[string]*ipLimiter
	stop            chan struct{}
	cleanupInterval time.Duration
	entryExpiry     time.Duration
}

type ipLimiter struct {
	limiter    *ratelimit.Limiter
	lastAccess time.Time
	mu         sync.Mutex
}

// serveHTTP is the actual rate limiting logic.
func (rl *rateLimiter) serveHTTP(w http.ResponseWriter, r *http.Request, next http.Handler) {
	logger := logging.L()
	clientIP := getClientIP(r)
	// Determine if request is authenticated
	authenticated := r.Header.Get("X-API-Key") != ""
	// Health endpoint is always unauthenticated
	if r.URL.Path == "/health" {
		authenticated = false
	}
	// Choose rate based on authentication
	rate := rl.unauthRate
	if authenticated {
		rate = rl.authRate
	}
	// Apply global limit first
	if rl.global != nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately to make Wait return immediately if no tokens
		err := rl.global.Wait(ctx)
		if err != nil {
			// Global rate limit exceeded
			logger.Warn("global rate limit exceeded",
				"client_ip", validation.SanitizeForLog(clientIP),
				"path", validation.SanitizeForLog(r.URL.Path),
				"method", r.Method,
			)
			w.Header().Set("Retry-After", "1")
			response.TooManyRequests(w, "rate limit exceeded")
			return
		}
	}
	// Apply per-IP limit if rate > 0
	if rate > 0 {
		limiter, _ := rl.getOrCreateLimiter(clientIP, rate)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := limiter.Wait(ctx)
		if err != nil {
			logger.Warn("per-IP rate limit exceeded",
				"client_ip", validation.SanitizeForLog(clientIP),
				"path", validation.SanitizeForLog(r.URL.Path),
				"method", r.Method,
				"authenticated", authenticated,
			)
			w.Header().Set("Retry-After", "1")
			response.TooManyRequests(w, "rate limit exceeded")
			return
		}
	}
	// Proceed to next handler
	next.ServeHTTP(w, r)
}

// getOrCreateLimiter returns the rate limiter for the given IP, creating it if necessary.
func (rl *rateLimiter) getOrCreateLimiter(ip string, rate float64) (*ratelimit.Limiter, bool) {
	burst := int(rate * rl.burstMultiplier)
	if burst < 1 {
		burst = 1
	}
	now := time.Now()
	// Load existing limiter
	if val, ok := rl.ipLimiters.Load(ip); ok {
		il := val.(*ipLimiter)
		il.mu.Lock()
		il.lastAccess = now
		il.mu.Unlock()
		return il.limiter, true
	}
	// Create new limiter
	limiter := ratelimit.NewLimiter(rate, burst)
	il := &ipLimiter{
		limiter:    limiter,
		lastAccess: now,
	}
	rl.ipLimiters.Store(ip, il)
	return limiter, false
}

// cleanup periodically removes stale IP limiter entries.
func (rl *rateLimiter) cleanup() {
	if rl.cleanupInterval == 0 {
		rl.cleanupInterval = time.Minute
	}
	if rl.entryExpiry == 0 {
		rl.entryExpiry = 10 * time.Minute
	}
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.ipLimiters.Range(func(key, value any) bool {
				il := value.(*ipLimiter)
				il.mu.Lock()
				expired := now.Sub(il.lastAccess) > rl.entryExpiry
				il.mu.Unlock()
				if expired {
					rl.ipLimiters.Delete(key)
				}
				return true
			})
		case <-rl.stop:
			return
		}
	}
}

// getClientIP extracts the client IP address from the request.
// It checks the X-Forwarded-For header (first IP in the list) and falls back to RemoteAddr.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (common when behind a proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs separated by commas
		// The first IP is the client's original IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			// Validate IP format (simple check)
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	// Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If splitting fails, assume the whole RemoteAddr is the IP
		return r.RemoteAddr
	}
	return host
}
