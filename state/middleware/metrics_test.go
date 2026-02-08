package middleware_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

// Simple metrics collector for testing
type testMetrics struct {
	stepCount    atomic.Int64
	totalDuration atomic.Int64 // nanoseconds
}

func (m *testMetrics) IncrementStepCount() {
	m.stepCount.Add(1)
}

func (m *testMetrics) RecordDuration(d time.Duration) {
	m.totalDuration.Add(int64(d))
}

func TestRecursiveMetrics(t *testing.T) {
	metrics := &testMetrics{}

	m := middleware.RecursiveMetrics(
		metrics.IncrementStepCount,
		metrics.RecordDuration,
	)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			time.Sleep(1 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			time.Sleep(1 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			time.Sleep(1 * time.Millisecond)
		}),
	)

	wrapped := state.WithMiddleware(pipeline, m)
	state.Run(context.Background(), wrapped)

	// Should have counted 3 steps
	count := metrics.stepCount.Load()
	if count < 3 {
		t.Errorf("expected at least 3 step counts, got %d", count)
	}

	// Should have recorded some duration
	duration := time.Duration(metrics.totalDuration.Load())
	if duration < 3*time.Millisecond {
		t.Errorf("expected at least 3ms total duration, got %v", duration)
	}
}
