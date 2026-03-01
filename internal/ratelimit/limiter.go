// Package ratelimit provides rate limiting with exponential backoff for API requests.
package ratelimit

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter controls the rate of API requests with support for backoff and per-route buckets.
type RateLimiter struct {
	mu                sync.Mutex
	defaultLimiter    *rate.Limiter
	routeLimiters     map[string]*rate.Limiter
	delayMs           int
	maxRetries        int
	backoffMultiplier float64
	batchSize         int
	paused            time.Time // if set, Wait blocks until this time
}

// Config holds rate limiter configuration.
type Config struct {
	DelayMs           int     // milliseconds between requests
	MaxRetries        int     // max retry count
	BackoffMultiplier float64 // exponential backoff factor
	BatchSize         int     // for Telegram batch deletes
}

// DefaultConfig returns sensible defaults for Discord (SPEC SS7).
func DefaultConfig() Config {
	return Config{
		DelayMs:           1000,
		MaxRetries:        5,
		BackoffMultiplier: 2.0,
		BatchSize:         100,
	}
}

// DefaultTelegramConfig returns sensible defaults for Telegram (SPEC SS7).
func DefaultTelegramConfig() Config {
	return Config{
		DelayMs:           500,
		MaxRetries:        5,
		BackoffMultiplier: 2.0,
		BatchSize:         100,
	}
}

// New creates a new RateLimiter with the given config.
func New(cfg Config) *RateLimiter {
	if cfg.DelayMs <= 0 {
		cfg.DelayMs = 1000
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.BackoffMultiplier <= 0 {
		cfg.BackoffMultiplier = 2.0
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	rps := 1000.0 / float64(cfg.DelayMs)
	limiter := rate.NewLimiter(rate.Limit(rps), 1)

	return &RateLimiter{
		defaultLimiter:    limiter,
		routeLimiters:     make(map[string]*rate.Limiter),
		delayMs:           cfg.DelayMs,
		maxRetries:        cfg.MaxRetries,
		backoffMultiplier: cfg.BackoffMultiplier,
		batchSize:         cfg.BatchSize,
	}
}

// SetRouteLimit configures a per-route rate limit.
func (rl *RateLimiter) SetRouteLimit(route string, delayMs int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rps := 1000.0 / float64(delayMs)
	rl.routeLimiters[route] = rate.NewLimiter(rate.Limit(rps), 1)
}

// Wait blocks until the next request is allowed for the default route.
// Returns an error if the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.WaitRoute(ctx, "")
}

// WaitRoute blocks until the next request is allowed for the given route.
func (rl *RateLimiter) WaitRoute(ctx context.Context, route string) error {
	// Check if we're in a rate-limit pause
	rl.mu.Lock()
	pauseUntil := rl.paused
	rl.mu.Unlock()

	if !pauseUntil.IsZero() && time.Now().Before(pauseUntil) {
		wait := time.Until(pauseUntil)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	limiter := rl.getLimiter(route)
	return limiter.Wait(ctx)
}

// HandleRateLimit is called when the API returns a 429/FLOOD_WAIT response.
// It pauses all requests for the given duration.
func (rl *RateLimiter) HandleRateLimit(retryAfter time.Duration) {
	slog.Debug("rate limit triggered, pausing requests", "retry_after", retryAfter)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.paused = time.Now().Add(retryAfter)
}

// MaxRetries returns the configured max retry count.
func (rl *RateLimiter) MaxRetries() int {
	return rl.maxRetries
}

// BackoffMultiplier returns the configured backoff multiplier.
func (rl *RateLimiter) BackoffMultiplier() float64 {
	return rl.backoffMultiplier
}

// BatchSize returns the configured batch size.
func (rl *RateLimiter) BatchSize() int {
	return rl.batchSize
}

// BackoffDuration calculates the backoff duration for the given attempt (0-indexed).
func (rl *RateLimiter) BackoffDuration(attempt int) time.Duration {
	delay := float64(rl.delayMs)
	for i := 0; i < attempt; i++ {
		delay *= rl.backoffMultiplier
	}
	return time.Duration(delay) * time.Millisecond
}

func (rl *RateLimiter) getLimiter(route string) *rate.Limiter {
	if route == "" {
		return rl.defaultLimiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if l, ok := rl.routeLimiters[route]; ok {
		return l
	}
	return rl.defaultLimiter
}
