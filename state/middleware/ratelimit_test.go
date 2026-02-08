package middleware_test

import (
	"context"
	"testing"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

// Simple rate limiter for testing
type testRateLimiter struct {
	delay time.Duration
}

func (r *testRateLimiter) Wait(ctx context.Context) error {
	select {
	case <-time.After(r.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRecursiveRateLimit(t *testing.T) {
	limiter := &testRateLimiter{delay: 10 * time.Millisecond}

	m := middleware.RecursiveRateLimit(limiter.Wait)

	start := time.Now()

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	wrapped := state.WithMiddleware(pipeline, m)
	state.Run(context.Background(), wrapped)

	elapsed := time.Since(start)

	// Should have waited at least 30ms (3 steps * 10ms each)
	if elapsed < 30*time.Millisecond {
		t.Errorf("expected at least 30ms elapsed, got %v", elapsed)
	}
}
