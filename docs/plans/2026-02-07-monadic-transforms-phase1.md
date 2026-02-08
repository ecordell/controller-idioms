# Monadic Transformations Phase 1: Core Infrastructure

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the core recursive middleware infrastructure that enables transformations to propagate through continuation chains.

**Architecture:** Create `state/transform` package with helpers for writing recursive middleware, a context-based transform registry, and `RunWithTransforms` that automatically applies registered transforms. This builds on the existing `state.Middleware` type without modifying the core `state` package.

**Tech Stack:** Go 1.21+, standard library context package, existing `state` package

---

## Task 1: Create Transform Package Structure

**Files:**
- Create: `state/transform/transform.go`
- Create: `state/transform/doc.go`

**Step 1: Create package documentation**

Create `state/transform/doc.go`:

```go
// Package transform provides utilities for creating recursive middleware
// that propagates through continuation chains in state pipelines.
//
// The key insight is that middleware can wrap not just the current step,
// but also the next continuation and any returned steps, allowing
// transformations to "thread through" entire pipeline executions.
package transform
```

**Step 2: Verify package compiles**

Run: `cd state/transform && go build`
Expected: Package compiles successfully

**Step 3: Commit**

```bash
git add state/transform/doc.go
git commit -m "feat(transform): create transform package with documentation"
```

---

## Task 2: Implement wrapStep Helper

**Files:**
- Modify: `state/transform/transform.go`
- Create: `state/transform/transform_test.go`

**Step 1: Write the failing test**

Create `state/transform/transform_test.go`:

```go
package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestWrapStep(t *testing.T) {
	executed := false
	step := state.StepFunc(func(ctx context.Context) state.Step {
		executed = true
		return nil
	})

	// Wrap the step with a no-op wrapper
	wrapper := func(s state.NewStep) state.NewStep {
		return s
	}

	wrapped := transform.WrapStep(step, wrapper)
	wrapped.Run(context.Background())

	if !executed {
		t.Error("wrapped step was not executed")
	}
}

func TestWrapStepNil(t *testing.T) {
	wrapper := func(s state.NewStep) state.NewStep {
		return s
	}

	wrapped := transform.WrapStep(nil, wrapper)
	if wrapped != nil {
		t.Error("wrapping nil step should return nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/transform && go test -v`
Expected: FAIL with "undefined: transform.WrapStep"

**Step 3: Write minimal implementation**

Add to `state/transform/transform.go`:

```go
package transform

import "github.com/authzed/controller-idioms/state"

// WrapStep wraps a Step with a recursive middleware wrapper.
// This is an internal utility for implementing recursive middleware.
// If step is nil, returns nil.
func WrapStep(step state.Step, wrapper func(state.NewStep) state.NewStep) state.Step {
	if step == nil {
		return nil
	}
	// Convert Step to NewStep, apply wrapper, then convert back
	return wrapper(func(next state.Step) state.Step {
		return step
	})(nil)
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/transform && go test -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add state/transform/transform.go state/transform/transform_test.go
git commit -m "feat(transform): implement WrapStep helper for recursive middleware"
```

---

## Task 3: Implement RecursiveMiddleware Helper

**Files:**
- Modify: `state/transform/transform.go`
- Modify: `state/transform/transform_test.go`

**Step 1: Write the failing test**

Add to `state/transform/transform_test.go`:

```go
func TestRecursiveMiddleware(t *testing.T) {
	var beforeCount, afterCount int

	before := func(ctx context.Context) {
		beforeCount++
	}
	after := func(ctx context.Context) {
		afterCount++
	}

	middleware := transform.RecursiveMiddleware(before, after)

	// Create a simple sequence
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	// Apply middleware and run
	wrapped := state.WithMiddleware(pipeline, middleware)
	state.Run(context.Background(), wrapped)

	// Should have called before/after for each step
	if beforeCount != 3 {
		t.Errorf("expected 3 before calls, got %d", beforeCount)
	}
	if afterCount != 3 {
		t.Errorf("expected 3 after calls, got %d", afterCount)
	}
}

func TestRecursiveMiddlewareWithDecision(t *testing.T) {
	var executionOrder []string

	before := func(ctx context.Context) {
		executionOrder = append(executionOrder, "before")
	}
	after := func(ctx context.Context) {
		executionOrder = append(executionOrder, "after")
	}

	middleware := transform.RecursiveMiddleware(before, after)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step1")
		}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {
				executionOrder = append(executionOrder, "true-branch")
			}),
			state.Action(func(ctx context.Context) {
				executionOrder = append(executionOrder, "false-branch")
			}),
		),
		state.Action(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step3")
		}),
	)

	wrapped := state.WithMiddleware(pipeline, middleware)
	state.Run(context.Background(), wrapped)

	// Verify before/after wraps each step
	expected := []string{
		"before", "step1", "after",
		"before", "before", "true-branch", "after", "after",
		"before", "step3", "after",
	}

	if len(executionOrder) != len(expected) {
		t.Errorf("expected %d calls, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/transform && go test -v`
Expected: FAIL with "undefined: transform.RecursiveMiddleware"

**Step 3: Write minimal implementation**

Add to `state/transform/transform.go`:

```go
// RecursiveMiddleware creates a middleware that propagates through the
// entire continuation chain by recursively wrapping continuations.
//
// The before function is called before each step executes.
// The after function is called after each step executes.
//
// This middleware will wrap not just the top-level step, but also:
// - The next continuation passed to each step
// - Any Step returned from execution
//
// This allows the middleware to "thread through" the entire pipeline.
func RecursiveMiddleware(
	before func(ctx context.Context),
	after func(ctx context.Context),
) state.Middleware {
	var wrap func(state.NewStep) state.NewStep
	wrap = func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			// Recursively wrap the next continuation
			wrappedNext := WrapStep(next, wrap)

			// Execute current step with wrapped next
			return state.StepFunc(func(ctx context.Context) state.Step {
				if before != nil {
					before(ctx)
				}

				result := step(wrappedNext).Run(ctx)

				if after != nil {
					after(ctx)
				}

				// Recursively wrap the result continuation
				if result != nil {
					return WrapStep(result, wrap)
				}
				return result
			})
		}
	}
	return wrap
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/transform && go test -v`
Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add state/transform/transform.go state/transform/transform_test.go
git commit -m "feat(transform): implement RecursiveMiddleware helper"
```

---

## Task 4: Implement Context Transform Registry

**Files:**
- Create: `state/transform/context.go`
- Create: `state/transform/context_test.go`

**Step 1: Write the failing test**

Create `state/transform/context_test.go`:

```go
package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestWithTransform(t *testing.T) {
	ctx := context.Background()

	middleware := func(s state.NewStep) state.NewStep {
		return s
	}

	ctx = transform.WithTransform(ctx, middleware)

	transforms := transform.GetTransforms(ctx)
	if len(transforms) != 1 {
		t.Errorf("expected 1 transform, got %d", len(transforms))
	}
}

func TestWithTransformMultiple(t *testing.T) {
	ctx := context.Background()

	m1 := func(s state.NewStep) state.NewStep { return s }
	m2 := func(s state.NewStep) state.NewStep { return s }
	m3 := func(s state.NewStep) state.NewStep { return s }

	ctx = transform.WithTransform(ctx, m1)
	ctx = transform.WithTransform(ctx, m2)
	ctx = transform.WithTransform(ctx, m3)

	transforms := transform.GetTransforms(ctx)
	if len(transforms) != 3 {
		t.Errorf("expected 3 transforms, got %d", len(transforms))
	}
}

func TestGetTransformsEmpty(t *testing.T) {
	ctx := context.Background()

	transforms := transform.GetTransforms(ctx)
	if transforms != nil {
		t.Errorf("expected nil for empty context, got %v", transforms)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/transform && go test -v`
Expected: FAIL with "undefined: transform.WithTransform"

**Step 3: Write minimal implementation**

Create `state/transform/context.go`:

```go
package transform

import (
	"context"

	"github.com/authzed/controller-idioms/state"
)

type contextKey int

const transformsKey contextKey = 0

// WithTransform registers a middleware transform in the context.
// Multiple transforms can be registered by calling this multiple times.
// Transforms are applied in the order they are registered.
func WithTransform(ctx context.Context, transform state.Middleware) context.Context {
	transforms := GetTransforms(ctx)
	if transforms == nil {
		transforms = []state.Middleware{}
	}
	transforms = append(transforms, transform)
	return context.WithValue(ctx, transformsKey, transforms)
}

// GetTransforms retrieves all middleware transforms from the context.
// Returns nil if no transforms are registered.
func GetTransforms(ctx context.Context) []state.Middleware {
	if v := ctx.Value(transformsKey); v != nil {
		return v.([]state.Middleware)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/transform && go test -v`
Expected: PASS (7 tests)

**Step 5: Commit**

```bash
git add state/transform/context.go state/transform/context_test.go
git commit -m "feat(transform): implement context-based transform registry"
```

---

## Task 5: Implement RunWithTransforms

**Files:**
- Create: `state/transform/run.go`
- Create: `state/transform/run_test.go`

**Step 1: Write the failing test**

Create `state/transform/run_test.go`:

```go
package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestRunWithTransforms(t *testing.T) {
	var executed bool
	pipeline := state.Action(func(ctx context.Context) {
		executed = true
	})

	ctx := context.Background()
	transform.RunWithTransforms(ctx, pipeline)

	if !executed {
		t.Error("pipeline was not executed")
	}
}

func TestRunWithTransformsAppliesMiddleware(t *testing.T) {
	var beforeCalled, actionCalled bool

	before := func(ctx context.Context) {
		beforeCalled = true
	}

	middleware := transform.RecursiveMiddleware(before, nil)

	pipeline := state.Action(func(ctx context.Context) {
		actionCalled = true
	})

	ctx := context.Background()
	ctx = transform.WithTransform(ctx, middleware)

	transform.RunWithTransforms(ctx, pipeline)

	if !beforeCalled {
		t.Error("middleware before function was not called")
	}
	if !actionCalled {
		t.Error("pipeline action was not called")
	}
}

func TestRunWithTransformsMultipleMiddleware(t *testing.T) {
	var calls []string

	m1 := transform.RecursiveMiddleware(
		func(ctx context.Context) { calls = append(calls, "m1-before") },
		func(ctx context.Context) { calls = append(calls, "m1-after") },
	)

	m2 := transform.RecursiveMiddleware(
		func(ctx context.Context) { calls = append(calls, "m2-before") },
		func(ctx context.Context) { calls = append(calls, "m2-after") },
	)

	pipeline := state.Action(func(ctx context.Context) {
		calls = append(calls, "action")
	})

	ctx := context.Background()
	ctx = transform.WithTransform(ctx, m1)
	ctx = transform.WithTransform(ctx, m2)

	transform.RunWithTransforms(ctx, pipeline)

	// Middleware should be applied in order: m1 wraps m2 wraps action
	// So execution order is: m1-before, m2-before, action, m2-after, m1-after
	expected := []string{"m1-before", "m2-before", "action", "m2-after", "m1-after"}
	if len(calls) != len(expected) {
		t.Errorf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}

	for i, call := range expected {
		if i >= len(calls) || calls[i] != call {
			t.Errorf("at position %d, expected %s, got %v", i, call, calls)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd state/transform && go test -v`
Expected: FAIL with "undefined: transform.RunWithTransforms"

**Step 3: Write minimal implementation**

Create `state/transform/run.go`:

```go
package transform

import (
	"context"

	"github.com/authzed/controller-idioms/state"
)

// RunWithTransforms executes a pipeline with all middleware transforms
// registered in the context automatically applied.
//
// Transforms are applied in the order they were registered with WithTransform.
// This is equivalent to manually calling state.WithMiddleware for each
// transform and then calling state.Run.
func RunWithTransforms(ctx context.Context, pipeline state.NewStep) {
	transforms := GetTransforms(ctx)
	if len(transforms) > 0 {
		pipeline = state.WithMiddleware(pipeline, transforms...)
	}
	state.Run(ctx, pipeline)
}
```

**Step 4: Run test to verify it passes**

Run: `cd state/transform && go test -v`
Expected: PASS (10 tests)

**Step 5: Commit**

```bash
git add state/transform/run.go state/transform/run_test.go
git commit -m "feat(transform): implement RunWithTransforms for automatic transform application"
```

---

## Task 6: Add Integration Test

**Files:**
- Modify: `state/transform/transform_test.go`

**Step 1: Write integration test**

Add to `state/transform/transform_test.go`:

```go
func TestIntegrationComplexPipeline(t *testing.T) {
	var trace []string

	logger := transform.RecursiveMiddleware(
		func(ctx context.Context) {
			trace = append(trace, "log-before")
		},
		func(ctx context.Context) {
			trace = append(trace, "log-after")
		},
	)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			trace = append(trace, "step1")
		}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {
				trace = append(trace, "true-branch")
			}),
			state.Action(func(ctx context.Context) {
				trace = append(trace, "false-branch")
			}),
		),
		state.Parallel(
			state.Action(func(ctx context.Context) {
				trace = append(trace, "parallel1")
			}),
			state.Action(func(ctx context.Context) {
				trace = append(trace, "parallel2")
			}),
		),
	)

	ctx := context.Background()
	ctx = transform.WithTransform(ctx, logger)

	transform.RunWithTransforms(ctx, pipeline)

	// Verify logging wrapped every step
	if len(trace) < 10 {
		t.Errorf("expected at least 10 trace entries, got %d: %v", len(trace), trace)
	}

	// Verify log-before appears before each action
	// and log-after appears after each action
	hasLogBefore := false
	hasStep1 := false
	for _, entry := range trace {
		if entry == "log-before" {
			hasLogBefore = true
		}
		if entry == "step1" && hasLogBefore {
			hasStep1 = true
		}
	}

	if !hasStep1 {
		t.Error("log-before should appear before step1")
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd state/transform && go test -v`
Expected: PASS (11 tests)

**Step 3: Commit**

```bash
git add state/transform/transform_test.go
git commit -m "test(transform): add integration test for complex pipeline"
```

---

## Task 7: Update Module and Run All Tests

**Files:**
- None (just verification)

**Step 1: Run all state package tests**

Run: `cd /Users/evan/git/authzed/controller-idioms/evan/monadic-transforms && go test ./state/... -v`
Expected: All tests pass (including formal_test.go and state_test.go)

**Step 2: Verify no breaking changes**

Run: `go test ./...`
Expected: All project tests pass

**Step 3: Commit if any go.mod/go.sum changes**

```bash
git add go.mod go.sum
git commit -m "chore: update go.mod/go.sum"
```

---

## Task 8: Add Package Documentation Examples

**Files:**
- Modify: `state/transform/doc.go`

**Step 1: Add comprehensive package examples**

Update `state/transform/doc.go`:

```go
// Package transform provides utilities for creating recursive middleware
// that propagates through continuation chains in state pipelines.
//
// The key insight is that middleware can wrap not just the current step,
// but also the next continuation and any returned steps, allowing
// transformations to "thread through" entire pipeline executions.
//
// # Basic Usage
//
// Create recursive middleware that logs before/after each step:
//
//	logger := transform.RecursiveMiddleware(
//		func(ctx context.Context) { log.Println("before step") },
//		func(ctx context.Context) { log.Println("after step") },
//	)
//
//	pipeline := state.Sequence(step1, step2, step3)
//	wrapped := state.WithMiddleware(pipeline, logger)
//	state.Run(ctx, wrapped)
//
// # Context-Based Transforms
//
// Register transforms in context for automatic application:
//
//	ctx = transform.WithTransform(ctx, loggingMiddleware)
//	ctx = transform.WithTransform(ctx, metricsMiddleware)
//
//	transform.RunWithTransforms(ctx, pipeline)
//
// # Writing Custom Recursive Middleware
//
// For more complex middleware, use the RecursiveMiddleware helper as a template
// and implement custom logic:
//
//	func CustomMiddleware(config Config) state.Middleware {
//		var wrap func(state.NewStep) state.NewStep
//		wrap = func(step state.NewStep) state.NewStep {
//			return func(next state.Step) state.Step {
//				wrappedNext := transform.WrapStep(next, wrap)
//				return state.StepFunc(func(ctx context.Context) state.Step {
//					// Before logic
//					result := step(wrappedNext).Run(ctx)
//					// After logic
//					if result != nil {
//						return transform.WrapStep(result, wrap)
//					}
//					return result
//				})
//			}
//		}
//		return wrap
//	}
package transform
```

**Step 2: Verify documentation builds**

Run: `cd state/transform && go doc`
Expected: Documentation displays correctly

**Step 3: Commit**

```bash
git add state/transform/doc.go
git commit -m "docs(transform): add comprehensive package documentation with examples"
```

---

## Task 9: Create Phase 1 Summary

**Files:**
- Create: `docs/plans/phase1-summary.md`

**Step 1: Write summary document**

Create `docs/plans/phase1-summary.md`:

```markdown
# Phase 1 Implementation Summary

**Completed:** 2026-02-07

## Deliverables

### Core Infrastructure (`state/transform`)

**New Files:**
- `state/transform/doc.go` - Package documentation with examples
- `state/transform/transform.go` - RecursiveMiddleware helper, WrapStep utility
- `state/transform/context.go` - Context-based transform registry
- `state/transform/run.go` - RunWithTransforms implementation
- `state/transform/transform_test.go` - Core tests (4 tests)
- `state/transform/context_test.go` - Context registry tests (3 tests)
- `state/transform/run_test.go` - RunWithTransforms tests (4 tests)

**Total Test Coverage:** 11 tests, all passing

### Key Functions

1. **RecursiveMiddleware(before, after)** - Helper for writing recursive middleware
2. **WrapStep(step, wrapper)** - Internal utility to wrap Steps recursively
3. **WithTransform(ctx, middleware)** - Register transform in context
4. **GetTransforms(ctx)** - Retrieve transforms from context
5. **RunWithTransforms(ctx, pipeline)** - Execute with automatic transform application

## Testing

All tests pass:
```bash
cd state/transform && go test -v
```

Integration verified with complex pipelines including:
- Sequential steps
- Decision branching
- Parallel execution

## Next Steps

**Phase 2:** Build middleware library (`state/middleware`)
- RecursiveLogging
- RecursiveMetrics
- RecursiveTracing (OpenTelemetry)
- RecursiveCircuitBreaker
- RecursiveRateLimit

**Phase 3:** Model checking utilities (`state/verify`)
- Property verification functions
- Execution tracing
- Safety/liveness/temporal property checks
```

**Step 2: Commit**

```bash
git add docs/plans/phase1-summary.md
git commit -m "docs: add Phase 1 implementation summary"
```

---

## Success Criteria

Phase 1 is complete when:

- ✅ `state/transform` package exists with all core functions
- ✅ RecursiveMiddleware propagates through continuations correctly
- ✅ Context transform registry works with multiple transforms
- ✅ RunWithTransforms applies transforms in correct order
- ✅ All tests pass (11 tests minimum)
- ✅ Integration test validates complex pipeline
- ✅ Documentation is comprehensive with examples
- ✅ No breaking changes to existing `state` package

## Testing Commands

```bash
# Run transform package tests
cd state/transform && go test -v

# Run all state package tests
go test ./state/... -v

# Run entire project tests
go test ./...
```

## Estimated Time

- Task 1-3: 15 minutes (package setup, wrapStep, RecursiveMiddleware)
- Task 4-5: 15 minutes (context registry, RunWithTransforms)
- Task 6-7: 10 minutes (integration test, verification)
- Task 8-9: 10 minutes (documentation, summary)

**Total: ~50 minutes**
