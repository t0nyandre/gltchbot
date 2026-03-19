package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter for a specific key.
type Limiter struct {
	mu         sync.Mutex
	rate       time.Duration // time between requests (1/rate per second)
	burst      int           // maximum burst size
	tokens     int           // current tokens
	lastUpdate time.Time     // last time tokens were replenished
}

// NewLimiter creates a new Limiter with the given rate (requests per second) and burst size.
func NewLimiter(ratePerSec float64, burst int) *Limiter {
	if burst < 1 {
		burst = 1
	}
	var rate time.Duration
	if ratePerSec <= 0 {
		rate = 0 // unlimited
	} else {
		rate = time.Duration(float64(time.Second) / ratePerSec)
	}
	return &Limiter{
		rate:       rate,
		burst:      burst,
		tokens:     burst,
		lastUpdate: time.Now(),
	}
}

// Wait blocks until a token is available for the limiter.
// If ctx is cancelled, returns ctx.Err().
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	// Replenish tokens based on elapsed time
	if l.rate > 0 {
		now := time.Now()
		elapsed := now.Sub(l.lastUpdate)
		newTokens := int(elapsed / l.rate)
		if newTokens > 0 {
			l.tokens += newTokens
			if l.tokens > l.burst {
				l.tokens = l.burst
			}
			l.lastUpdate = now
		}
	}
	// If tokens available, consume one and return
	if l.tokens > 0 {
		l.tokens--
		l.mu.Unlock()
		return nil
	}
	// Need to wait for next token
	waitTime := l.rate
	// Schedule when the next token will be available
	nextAvailable := l.lastUpdate.Add(l.rate)
	waitTime = nextAvailable.Sub(time.Now())
	if waitTime < 0 {
		waitTime = 0
	}
	l.lastUpdate = nextAvailable
	l.mu.Unlock()
	timer := time.NewTimer(waitTime)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetRate updates the rate and burst of the limiter.
func (l *Limiter) SetRate(ratePerSec float64, burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if burst < 1 {
		burst = 1
	}
	l.burst = burst
	if ratePerSec <= 0 {
		l.rate = 0
	} else {
		l.rate = time.Duration(float64(time.Second) / ratePerSec)
	}
	// Adjust tokens to not exceed new burst
	if l.tokens > burst {
		l.tokens = burst
	}
}
