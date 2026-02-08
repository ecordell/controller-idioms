package middleware_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

// TestMultipleMiddleware tests combining multiple middleware on a single pipeline.
// This ensures middleware compose correctly and don't interfere with each other.
func TestMultipleMiddleware(t *testing.T) {
	// Set up test infrastructure
	logger := &testLogger{}
	metrics := &testMetrics{}

	var stepExecutions atomic.Int64

	// Create a pipeline with 3 steps
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			stepExecutions.Add(1)
			time.Sleep(1 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			stepExecutions.Add(1)
			time.Sleep(1 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			stepExecutions.Add(1)
			time.Sleep(1 * time.Millisecond)
		}),
	)

	// Apply multiple middleware
	wrapped := state.WithMiddleware(
		pipeline,
		middleware.RecursiveLogging(logger.Log),
		middleware.RecursiveMetrics(
			metrics.IncrementStepCount,
			metrics.RecordDuration,
		),
	)

	// Execute the wrapped pipeline
	state.Run(context.Background(), wrapped)

	// Verify all steps executed
	if executions := stepExecutions.Load(); executions != 3 {
		t.Errorf("expected 3 step executions, got %d", executions)
	}

	// Verify logging middleware worked (at least 2 logs - before and after)
	if len(logger.logs) < 2 {
		t.Errorf("expected at least 2 log entries, got %d: %v", len(logger.logs), logger.logs)
	}

	hasExecutingStep := false
	for _, log := range logger.logs {
		if strings.Contains(log, "executing step") || strings.Contains(log, "completed step") {
			hasExecutingStep = true
			break
		}
	}
	if !hasExecutingStep {
		t.Error("expected 'executing step' or 'completed step' in logs")
	}

	// Verify metrics middleware worked
	if count := metrics.stepCount.Load(); count < 1 {
		t.Errorf("expected at least 1 step count, got %d", count)
	}

	if duration := time.Duration(metrics.totalDuration.Load()); duration < 3*time.Millisecond {
		t.Errorf("expected at least 3ms total duration, got %v", duration)
	}
}

// TestMiddlewareWithCircuitBreaker tests that circuit breaker works with other middleware.
func TestMiddlewareWithCircuitBreaker(t *testing.T) {
	logger := &testLogger{}
	var executionCount atomic.Int64

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			executionCount.Add(1)
		}),
		state.Action(func(ctx context.Context) {
			executionCount.Add(1)
		}),
	)

	// Apply circuit breaker and logging
	wrapped := state.WithMiddleware(
		pipeline,
		middleware.RecursiveCircuitBreaker(2), // Open after 2 failures
		middleware.RecursiveLogging(logger.Log),
	)

	// First failure
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1()
	state.Run(ctx1, wrapped)

	// Second failure
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	state.Run(ctx2, wrapped)

	// Third execution - circuit should be open
	beforeThird := executionCount.Load()
	state.Run(context.Background(), wrapped)
	afterThird := executionCount.Load()

	if afterThird != beforeThird {
		t.Errorf("expected circuit open (no new executions), but got %d new executions", afterThird-beforeThird)
	}

	// Verify logging still happened
	if len(logger.logs) == 0 {
		t.Error("expected some log entries even with circuit breaker")
	}
}

// TestMiddlewareWithRateLimit tests that rate limiting works with other middleware.
func TestMiddlewareWithRateLimit(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}

	var delays atomic.Int64
	rateLimitFunc := func(ctx context.Context) error {
		delays.Add(1)
		time.Sleep(5 * time.Millisecond)
		return nil
	}

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	// Apply rate limit, logging, and metrics
	wrapped := state.WithMiddleware(
		pipeline,
		middleware.RecursiveRateLimit(rateLimitFunc),
		middleware.RecursiveLogging(logger.Log),
		middleware.RecursiveMetrics(
			metrics.IncrementStepCount,
			metrics.RecordDuration,
		),
	)

	start := time.Now()
	state.Run(context.Background(), wrapped)
	elapsed := time.Since(start)

	// Verify rate limiting happened (at least 1 delay)
	if delays.Load() < 1 {
		t.Errorf("expected at least 1 rate limit delay, got %d", delays.Load())
	}

	if elapsed < 5*time.Millisecond {
		t.Errorf("expected at least 5ms elapsed due to rate limiting, got %v", elapsed)
	}

	// Verify logging still worked
	if len(logger.logs) < 2 {
		t.Errorf("expected at least 2 log entries, got %d", len(logger.logs))
	}

	// Verify metrics still worked
	if count := metrics.stepCount.Load(); count < 1 {
		t.Errorf("expected at least 1 step count, got %d", count)
	}
}

// TestMiddlewareOrder verifies that middleware is applied in the correct order.
// WithMiddleware applies middleware right-to-left, so the last middleware is outermost.
func TestMiddlewareOrder(t *testing.T) {
	var order []string

	middleware1 := func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			return state.StepFunc(func(ctx context.Context) state.Step {
				order = append(order, "m1-before")
				result := step(next).Run(ctx)
				order = append(order, "m1-after")
				return result
			})
		}
	}

	middleware2 := func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			return state.StepFunc(func(ctx context.Context) state.Step {
				order = append(order, "m2-before")
				result := step(next).Run(ctx)
				order = append(order, "m2-after")
				return result
			})
		}
	}

	pipeline := state.Action(func(ctx context.Context) {
		order = append(order, "action")
	})

	// Apply middleware: m1, then m2
	// WithMiddleware applies right-to-left, so m2 is applied first (outermost)
	// Expected order: m1-before, m2-before, action, m2-after, m1-after
	wrapped := state.WithMiddleware(pipeline, middleware1, middleware2)
	state.Run(context.Background(), wrapped)

	expected := []string{"m1-before", "m2-before", "action", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(order), order)
	}

	for i, exp := range expected {
		if order[i] != exp {
			t.Errorf("at position %d: expected %q, got %q", i, exp, order[i])
		}
	}
}
