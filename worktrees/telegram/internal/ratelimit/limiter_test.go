package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	rl := New(DefaultConfig())
	if rl.MaxRetries() != 5 {
		t.Errorf("expected max retries 5, got %d", rl.MaxRetries())
	}
	if rl.BackoffMultiplier() != 2.0 {
		t.Errorf("expected backoff 2.0, got %f", rl.BackoffMultiplier())
	}
	if rl.BatchSize() != 100 {
		t.Errorf("expected batch size 100, got %d", rl.BatchSize())
	}
}

func TestWaitRespectsContext(t *testing.T) {
	rl := New(Config{DelayMs: 1000, MaxRetries: 1, BackoffMultiplier: 2, BatchSize: 1})

	ctx, cancel := context.WithCancel(context.Background())
	// First call should succeed
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first wait should succeed: %v", err)
	}

	// Cancel context before next wait
	cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Error("expected error after context cancellation")
	}
}

func TestWaitDelaysBetweenCalls(t *testing.T) {
	// 100ms delay = 10 RPS
	rl := New(Config{DelayMs: 100, MaxRetries: 1, BackoffMultiplier: 2, BatchSize: 1})
	ctx := context.Background()

	start := time.Now()
	_ = rl.Wait(ctx)
	_ = rl.Wait(ctx)
	elapsed := time.Since(start)

	// Should take at least ~100ms for 2 calls with 100ms delay
	if elapsed < 80*time.Millisecond {
		t.Errorf("expected at least 80ms between calls, got %v", elapsed)
	}
}

func TestHandleRateLimit(t *testing.T) {
	rl := New(Config{DelayMs: 10, MaxRetries: 1, BackoffMultiplier: 2, BatchSize: 1})

	// Pause for 200ms
	rl.HandleRateLimit(200 * time.Millisecond)

	ctx := context.Background()
	start := time.Now()
	_ = rl.Wait(ctx)
	elapsed := time.Since(start)

	if elapsed < 150*time.Millisecond {
		t.Errorf("expected pause of ~200ms, got %v", elapsed)
	}
}

func TestHandleRateLimitContextCancel(t *testing.T) {
	rl := New(Config{DelayMs: 10, MaxRetries: 1, BackoffMultiplier: 2, BatchSize: 1})

	rl.HandleRateLimit(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected context error during rate limit pause")
	}
}

func TestPerRouteLimiter(t *testing.T) {
	rl := New(Config{DelayMs: 10, MaxRetries: 1, BackoffMultiplier: 2, BatchSize: 1})
	rl.SetRouteLimit("slow", 200)

	ctx := context.Background()

	start := time.Now()
	_ = rl.WaitRoute(ctx, "slow")
	_ = rl.WaitRoute(ctx, "slow")
	elapsed := time.Since(start)

	if elapsed < 150*time.Millisecond {
		t.Errorf("expected slow route to take ~200ms, got %v", elapsed)
	}
}

func TestBackoffDuration(t *testing.T) {
	rl := New(Config{DelayMs: 100, MaxRetries: 3, BackoffMultiplier: 2, BatchSize: 1})

	d0 := rl.BackoffDuration(0)
	if d0 != 100*time.Millisecond {
		t.Errorf("attempt 0: expected 100ms, got %v", d0)
	}

	d1 := rl.BackoffDuration(1)
	if d1 != 200*time.Millisecond {
		t.Errorf("attempt 1: expected 200ms, got %v", d1)
	}

	d2 := rl.BackoffDuration(2)
	if d2 != 400*time.Millisecond {
		t.Errorf("attempt 2: expected 400ms, got %v", d2)
	}
}
