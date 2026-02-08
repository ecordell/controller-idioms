# Monadic Transformations Phase 2: Built-in Middleware Library

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a library of common recursive middleware implementations (logging, metrics, tracing, circuit breaker, rate limiting) in the `state/middleware` package.

**Architecture:** Create `state/middleware` package with concrete implementations of recursive middleware for common cross-cutting concerns. Each middleware uses the `transform.RecursiveMiddleware` helper from Phase 1 or implements the recursive wrapping pattern directly for more complex cases.

**Tech Stack:** Go 1.21+, OpenTelemetry for tracing, standard library for metrics/logging

---

## Task 1: Create Middleware Package Structure

**Files:**
- Create: `state/middleware/middleware.go`
- Create: `state/middleware/doc.go`

**Step 1: Create package documentation**

Create `state/middleware/doc.go`:

```go
// Package middleware provides common recursive middleware implementations
// for cross-cutting concerns in state pipelines.
//
// All middleware in this package uses recursive wrapping to propagate
// through entire pipeline executions, including continuations and branches.
//
// # Available Middleware
//
// - RecursiveLogging: Log before/after each step
// - RecursiveMetrics: Record timing and execution counts
// - RecursiveTracing: OpenTelemetry distributed tracing
// - RecursiveCircuitBreaker: Stop execution after N failures
// - RecursiveRateLimit: Throttle execution between steps
//
// # Usage
//
//	import (
//		"github.com/authzed/controller-idioms/state"
//		"github.com/authzed/controller-idioms/state/middleware"
//		"github.com/authzed/controller-idioms/state/transform"
//	)
//
//	pipeline := state.Sequence(step1, step2, step3)
//
//	// Apply explicit middleware
//	logged := state.WithMiddleware(pipeline, middleware.RecursiveLogging(logger))
//	state.Run(ctx, logged)
//
//	// Or use context-based automatic application
//	ctx = transform.WithTransform(ctx, middleware.RecursiveLogging(logger))
//	ctx = transform.WithTransform(ctx, middleware.RecursiveMetrics(collector))
//	transform.RunWithTransforms(ctx, pipeline)
package middleware
```

**Step 2: Create empty middleware.go**

Create `state/middleware/middleware.go`:

```go
package middleware
```

**Step 3: Verify package compiles**

Run: `cd state/middleware && go build`
Expected: Package compiles successfully

**Step 4: Commit**

```bash
git add state/middleware/doc.go state/middleware/middleware.go
git commit -m "feat(middleware): create middleware package with documentation"
```

---

## Task 2: Implement RecursiveLogging

**Files:**
- Create: `state/middleware/logging.go`
- Create: `state/middleware/logging_test.go`

**Step 1: Write the failing test**

Create `state/middleware/logging_test.go`:

```go
package middleware_test

import (
	"context"
	"strings"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

// Simple logger for testing
type testLogger struct {
	logs []string
}

func (l *testLogger) Log(msg string) {
	l.logs = append(l.logs, msg)
}

func TestRecursiveLogging(t *testing.T) {
	logger := &testLogger{}

	m := middleware.RecursiveLogging(logger.Log)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	wrapped := state.WithMiddleware(pipeline, m)
	state.Run(context.Background(), wrapped)

	// Should have logged before/after each step (6 logs minimum)
	if len(logger.logs) < 6 {
		t.Errorf("expected at least 6 log entries, got %d: %v", len(logger.logs), logger.logs)
	}

	// Verify "executing step" appears in logs
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
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/middleware && go test -v`
Expected: FAIL with "undefined: middleware.RecursiveLogging"

**Step 3: Write minimal implementation**

Create `state/middleware/logging.go`:

```go
package middleware

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveLogging creates middleware that logs before and after each step execution.
// The logFunc receives a formatted log message for each step.
//
// Each step is assigned a unique ID for tracing through the pipeline.
func RecursiveLogging(logFunc func(string)) state.Middleware {
	var stepCounter atomic.Int64

	return transform.RecursiveMiddleware(
		func(ctx context.Context) {
			id := stepCounter.Add(1)
			logFunc(fmt.Sprintf("[step-%d] executing step", id))
		},
		func(ctx context.Context) {
			id := stepCounter.Load()
			logFunc(fmt.Sprintf("[step-%d] completed step", id))
		},
	)
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/middleware && go test -v`
Expected: PASS (1 test)

**Step 5: Commit**

```bash
git add state/middleware/logging.go state/middleware/logging_test.go
git commit -m "feat(middleware): implement RecursiveLogging"
```

---

## Task 3: Implement RecursiveMetrics

**Files:**
- Create: `state/middleware/metrics.go`
- Create: `state/middleware/metrics_test.go`

**Step 1: Write the failing test**

Create `state/middleware/metrics_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd state/middleware && go test -v`
Expected: FAIL with "undefined: middleware.RecursiveMetrics"

**Step 3: Write minimal implementation**

Create `state/middleware/metrics.go`:

```go
package middleware

import (
	"context"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveMetrics creates middleware that records metrics for each step execution.
// 
// incrementFunc is called before each step (for counting executions).
// recordDurationFunc is called after each step with the execution duration.
func RecursiveMetrics(
	incrementFunc func(),
	recordDurationFunc func(time.Duration),
) state.Middleware {
	type metricsKey struct{}
	
	return transform.RecursiveMiddleware(
		func(ctx context.Context) {
			if incrementFunc != nil {
				incrementFunc()
			}
			// Store start time in context
			ctx = context.WithValue(ctx, metricsKey{}, time.Now())
		},
		func(ctx context.Context) {
			if recordDurationFunc != nil {
				if start, ok := ctx.Value(metricsKey{}).(time.Time); ok {
					duration := time.Since(start)
					recordDurationFunc(duration)
				}
			}
		},
	)
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/middleware && go test -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add state/middleware/metrics.go state/middleware/metrics_test.go
git commit -m "feat(middleware): implement RecursiveMetrics"
```

---

## Task 4: Implement RecursiveCircuitBreaker

**Files:**
- Create: `state/middleware/circuitbreaker.go`
- Create: `state/middleware/circuitbreaker_test.go`

**Step 1: Write the failing test**

Create `state/middleware/circuitbreaker_test.go`:

```go
package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

func TestRecursiveCircuitBreaker(t *testing.T) {
	var executionCount int

	m := middleware.RecursiveCircuitBreaker(2) // Open after 2 failures

	// Create a pipeline that fails on the first two executions
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			executionCount++
			if executionCount <= 2 {
				// Simulate failure by canceling context
				ctx = context.WithValue(ctx, struct{}{}, errors.New("simulated failure"))
			}
		}),
		state.Action(func(ctx context.Context) {
			executionCount++
		}),
		state.Action(func(ctx context.Context) {
			executionCount++
		}),
	)

	wrapped := state.WithMiddleware(pipeline, m)
	
	// First execution - should reach first action
	state.Run(context.Background(), wrapped)
	if executionCount != 1 {
		t.Errorf("first execution: expected 1 action, got %d", executionCount)
	}

	// Second execution - should reach first action then stop
	state.Run(context.Background(), wrapped)
	if executionCount != 2 {
		t.Errorf("second execution: expected 2 total actions, got %d", executionCount)
	}

	// Third execution - circuit should be open, no actions execute
	state.Run(context.Background(), wrapped)
	if executionCount != 2 {
		t.Errorf("third execution: expected circuit open (2 total), got %d", executionCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/middleware && go test -v -run TestRecursiveCircuitBreaker`
Expected: FAIL with "undefined: middleware.RecursiveCircuitBreaker"

**Step 3: Write minimal implementation**

Create `state/middleware/circuitbreaker.go`:

```go
package middleware

import (
	"context"
	"sync"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveCircuitBreaker creates middleware that stops execution after
// maxFailures error occurrences.
//
// An error is counted when the context has an error (ctx.Err() != nil).
// Once the circuit is open, all subsequent steps are skipped.
func RecursiveCircuitBreaker(maxFailures int) state.Middleware {
	var (
		failures int
		mu       sync.Mutex
	)

	var wrap func(state.NewStep) state.NewStep
	wrap = func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			wrappedNext := transform.WrapStep(next, wrap)

			return state.StepFunc(func(ctx context.Context) state.Step {
				// Check if circuit is open
				mu.Lock()
				isOpen := failures >= maxFailures
				mu.Unlock()

				if isOpen {
					// Circuit open - skip execution
					return nil
				}

				// Execute step
				result := step(wrappedNext).Run(ctx)

				// Check for error
				if ctx.Err() != nil {
					mu.Lock()
					failures++
					mu.Unlock()
				}

				// Wrap result
				if result != nil {
					return transform.WrapStep(result, wrap)
				}
				return result
			})
		}
	}
	return wrap
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/middleware && go test -v -run TestRecursiveCircuitBreaker`
Expected: PASS

**Step 5: Commit**

```bash
git add state/middleware/circuitbreaker.go state/middleware/circuitbreaker_test.go
git commit -m "feat(middleware): implement RecursiveCircuitBreaker"
```

---

## Task 5: Implement RecursiveRateLimit

**Files:**
- Create: `state/middleware/ratelimit.go`
- Create: `state/middleware/ratelimit_test.go`

**Step 1: Write the failing test**

Create `state/middleware/ratelimit_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd state/middleware && go test -v -run TestRecursiveRateLimit`
Expected: FAIL with "undefined: middleware.RecursiveRateLimit"

**Step 3: Write minimal implementation**

Create `state/middleware/ratelimit.go`:

```go
package middleware

import (
	"context"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveRateLimit creates middleware that rate limits step execution.
//
// The waitFunc is called before each step execution and should block
// until the step is allowed to proceed. It should respect context
// cancellation.
//
// Example waitFunc implementations:
// - time.Sleep wrapper
// - golang.org/x/time/rate.Limiter.Wait
// - Custom token bucket implementation
func RecursiveRateLimit(waitFunc func(context.Context) error) state.Middleware {
	return transform.RecursiveMiddleware(
		func(ctx context.Context) {
			if waitFunc != nil {
				// Ignore error - if context is canceled, the step will handle it
				_ = waitFunc(ctx)
			}
		},
		nil, // No after logic needed
	)
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/middleware && go test -v -run TestRecursiveRateLimit`
Expected: PASS

**Step 5: Commit**

```bash
git add state/middleware/ratelimit.go state/middleware/ratelimit_test.go
git commit -m "feat(middleware): implement RecursiveRateLimit"
```

---

## Task 6: Add Integration Test

**Files:**
- Modify: `state/middleware/middleware_test.go`

**Step 1: Create integration test file**

Create `state/middleware/middleware_test.go`:

```go
package middleware_test

import (
	"context"
	"testing"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestIntegrationMultipleMiddleware(t *testing.T) {
	// Set up test infrastructure
	logger := &testLogger{}
	metrics := &testMetrics{}
	limiter := &testRateLimiter{delay: 5 * time.Millisecond}

	// Create multiple middleware
	logging := middleware.RecursiveLogging(logger.Log)
	metricsM := middleware.RecursiveMetrics(
		metrics.IncrementStepCount,
		metrics.RecordDuration,
	)
	rateLimit := middleware.RecursiveRateLimit(limiter.Wait)

	// Create a pipeline
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {}),
			state.Action(func(ctx context.Context) {}),
		),
		state.Action(func(ctx context.Context) {}),
	)

	// Apply all middleware using context-based transforms
	ctx := context.Background()
	ctx = transform.WithTransform(ctx, logging)
	ctx = transform.WithTransform(ctx, metricsM)
	ctx = transform.WithTransform(ctx, rateLimit)

	start := time.Now()
	transform.RunWithTransforms(ctx, pipeline)
	elapsed := time.Since(start)

	// Verify logging worked
	if len(logger.logs) < 6 {
		t.Errorf("expected at least 6 log entries, got %d", len(logger.logs))
	}

	// Verify metrics worked
	stepCount := metrics.stepCount.Load()
	if stepCount < 3 {
		t.Errorf("expected at least 3 steps counted, got %d", stepCount)
	}

	totalDuration := time.Duration(metrics.totalDuration.Load())
	if totalDuration == 0 {
		t.Error("expected non-zero total duration")
	}

	// Verify rate limiting worked
	if elapsed < 15*time.Millisecond { // At least 3 steps * 5ms
		t.Errorf("expected at least 15ms elapsed due to rate limiting, got %v", elapsed)
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd state/middleware && go test -v -run TestIntegration`
Expected: PASS

**Step 3: Commit**

```bash
git add state/middleware/middleware_test.go
git commit -m "test(middleware): add integration test with multiple middleware"
```

---

## Task 7: Run All Tests

**Files:**
- None (verification only)

**Step 1: Run all middleware tests**

Run: `cd state/middleware && go test -v`
Expected: All tests pass

**Step 2: Run all state package tests**

Run: `go test ./state/... -v`
Expected: All tests pass (transform + middleware + core state)

**Step 3: Report test count**

Count total tests and verify no regressions

---

## Task 8: Update Package Documentation

**Files:**
- Modify: `state/middleware/doc.go`

**Step 1: Add comprehensive examples**

Update `state/middleware/doc.go` with detailed examples for each middleware type, showing both explicit and context-based usage patterns.

**Step 2: Verify documentation**

Run: `cd state/middleware && go doc`
Expected: Documentation displays correctly

**Step 3: Commit**

```bash
git add state/middleware/doc.go
git commit -m "docs(middleware): add comprehensive usage examples"
```

---

## Task 9: Create Phase 2 Summary

**Files:**
- Create: `docs/plans/phase2-summary.md`

**Step 1: Write summary document**

Create `docs/plans/phase2-summary.md`:

```markdown
# Phase 2 Implementation Summary

**Completed:** 2026-02-07

## Deliverables

### Built-in Middleware Library (`state/middleware`)

**New Files:**
- `state/middleware/doc.go` - Package documentation
- `state/middleware/middleware.go` - Package structure
- `state/middleware/logging.go` - RecursiveLogging implementation
- `state/middleware/metrics.go` - RecursiveMetrics implementation
- `state/middleware/circuitbreaker.go` - RecursiveCircuitBreaker implementation
- `state/middleware/ratelimit.go` - RecursiveRateLimit implementation
- `state/middleware/logging_test.go` - Logging tests
- `state/middleware/metrics_test.go` - Metrics tests
- `state/middleware/circuitbreaker_test.go` - Circuit breaker tests
- `state/middleware/ratelimit_test.go` - Rate limit tests
- `state/middleware/middleware_test.go` - Integration tests

**Total Test Coverage:** ~6-8 tests, all passing

### Middleware Implementations

1. **RecursiveLogging** - Logs before/after each step with unique IDs
2. **RecursiveMetrics** - Records execution counts and durations
3. **RecursiveCircuitBreaker** - Stops execution after N failures
4. **RecursiveRateLimit** - Throttles execution between steps

## Testing

All tests pass:
```bash
cd state/middleware && go test -v
```

Integration verified with multiple middleware applied together.

## Next Steps

**Phase 3:** Model checking utilities (`state/verify`)
- Property verification functions
- Execution tracing
- Safety/liveness/temporal property checks
```

**Step 2: Commit**

```bash
git add docs/plans/phase2-summary.md
git commit -m "docs: add Phase 2 implementation summary"
```

---

## Success Criteria

Phase 2 is complete when:

- ✅ `state/middleware` package exists with 5 middleware implementations
- ✅ RecursiveLogging works with custom log functions
- ✅ RecursiveMetrics tracks counts and durations
- ✅ RecursiveCircuitBreaker stops after max failures
- ✅ RecursiveRateLimit throttles execution
- ✅ All tests pass (6-8 tests minimum)
- ✅ Integration test validates multiple middleware together
- ✅ Documentation is comprehensive
- ✅ No breaking changes to existing state/transform packages

## Testing Commands

```bash
# Run middleware package tests
cd state/middleware && go test -v

# Run all state package tests
go test ./state/... -v

# Run entire project tests
go test ./...
```

## Estimated Time

- Task 1: 5 minutes (package setup)
- Task 2-5: 40 minutes (4 middleware × 10 min each)
- Task 6-7: 10 minutes (integration test, verification)
- Task 8-9: 10 minutes (documentation, summary)

**Total: ~65 minutes**
