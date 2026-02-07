# Monadic Pipeline Transformations and Verification

**Date:** 2026-02-07  
**Status:** Design  
**Author:** Evan (with Claude)

## Overview

This document describes how to leverage the monadic structure of the `state` package to provide powerful transformation and verification capabilities. The key insight is that continuation-passing style enables recursive middleware that can "thread through" entire pipeline executions without requiring structural inspection of the pipeline itself.

## Motivation

The `state` package provides a clean monadic foundation with proper category theory semantics. However, this formal foundation should enable practical capabilities:

1. **Developer Productivity** - Easy addition of cross-cutting concerns (logging, metrics, tracing)
2. **Runtime Flexibility** - Dynamic behavior modification (feature flags, A/B testing, debug modes)
3. **Testing & Debugging** - Property verification, execution tracing, and replay capabilities

## Core Insight: Recursive Middleware via Continuation Wrapping

The `state` package uses continuation-passing style where each step receives the next continuation:

```go
type NewStep func(next Step) Step
```

Middleware can wrap not just the current step, but also:
1. The `next` continuation passed to it
2. Any `Step` returned from execution

This enables middleware to propagate recursively through the entire execution chain:

```go
func RecursiveMiddleware(/* config */) Middleware {
    var wrap func(NewStep) NewStep
    wrap = func(step NewStep) NewStep {
        return func(next Step) Step {
            // 1. Wrap the next continuation
            wrappedNext := wrapStep(next, wrap)
            
            // 2. Execute current step with wrapped next
            return state.StepFunc(func(ctx context.Context) Step {
                // Before logic
                result := step(wrappedNext).Run(ctx)
                // After logic
                
                // 3. Wrap the result continuation
                if result != nil {
                    return wrapStep(result, wrap)
                }
                return result
            })
        }
    }
    return wrap
}
```

This pattern enables transformations like "inject logging between every step" without needing to inspect the pipeline structure.

## Architecture

### Package Structure

Keep `state` pure and provide extensions in separate packages:

```
state/               # Pure monadic pipeline primitives
├── state.go         # Core types and combinators (unchanged)
├── state_test.go    # Functional tests
└── formal_test.go   # Category theory validation

state/transform/     # Recursive middleware infrastructure
├── transform.go     # RecursiveMiddleware helper, wrapStep utility
├── context.go       # Context-based transform registry
├── run.go           # RunWithTransforms
└── transform_test.go

state/middleware/    # Built-in recursive middleware
├── logging.go       # RecursiveLogging
├── metrics.go       # RecursiveMetrics  
├── tracing.go       # RecursiveTracing (OpenTelemetry)
├── circuitbreaker.go # RecursiveCircuitBreaker
├── ratelimit.go     # RecursiveRateLimit
└── middleware_test.go

state/verify/        # Model checking utilities
├── verify.go        # Property verification functions
├── trace.go         # ExecutionTrace, RecursiveTracing
└── verify_test.go
```

### Core Components

#### 1. Transform Infrastructure (`state/transform`)

**RecursiveMiddleware Helper:**
```go
// Helper for writing recursive middleware
func RecursiveMiddleware(
    before func(ctx context.Context),
    after func(ctx context.Context),
) state.Middleware

// Internal utility to wrap Step with recursive middleware
func wrapStep(step state.Step, wrapper func(state.NewStep) state.NewStep) state.Step
```

**Context-Based Transform Registry:**
```go
// Register transforms in context
func WithTransform(ctx context.Context, transform state.Middleware) context.Context

// Enhanced Run that applies context transforms
func RunWithTransforms(ctx context.Context, pipeline state.NewStep)
```

**Convenience Helpers:**
```go
func WithLogging(ctx context.Context, logger Logger) context.Context
func WithMetrics(ctx context.Context, collector Metrics) context.Context
func WithTracing(ctx context.Context, tracer trace.Tracer) context.Context
func WithCircuitBreaker(ctx context.Context, maxFailures int) context.Context
func WithRateLimit(ctx context.Context, limiter RateLimiter) context.Context
```

#### 2. Built-in Middleware Library (`state/middleware`)

**Logging:**
```go
func RecursiveLogging(logger Logger) state.Middleware
```
- Logs before/after every step execution
- Assigns unique step IDs for tracing
- Configurable log levels

**Metrics:**
```go
func RecursiveMetrics(collector MetricsCollector) state.Middleware
```
- Records timing for each step
- Counts step executions
- Tracks errors/panics

**Tracing (OpenTelemetry):**
```go
func RecursiveTracing(tracer trace.Tracer) state.Middleware
```
- Creates spans for each step
- Propagates trace context
- Records step metadata

**Circuit Breaker:**
```go
func RecursiveCircuitBreaker(maxFailures int) state.Middleware
```
- Stops execution after N failures
- Configurable failure threshold
- Thread-safe failure counting

**Rate Limiting:**
```go
func RecursiveRateLimit(limiter RateLimiter) state.Middleware
```
- Throttles execution between steps
- Respects context cancellation
- Pluggable rate limiter interface

#### 3. Model Checking Utilities (`state/verify`)

**Property Verification:**
```go
// Safety properties
func VerifyNoPanic(pipeline state.NewStep) error
func VerifyResourceInvariant(pipeline state.NewStep, check func(ctx) bool) error

// Liveness properties
func VerifyEventuallyTerminates(pipeline state.NewStep, timeout time.Duration) error
func VerifyProgress(pipeline state.NewStep, maxSteps int) error

// Temporal/trace properties
func VerifyOrdering(pipeline state.NewStep, mustBefore, mustAfter string) error
```

**Execution Tracing:**
```go
type ExecutionTrace struct {
    Steps []StepTrace
}

type StepTrace struct {
    Type      string
    Branch    string
    Timestamp time.Time
    Context   map[string]any
}

func RecursiveTracing() (state.Middleware, *ExecutionTrace)
```

## Usage Examples

### Example 1: Explicit Transformation

```go
import (
    "github.com/authzed/controller-idioms/state"
    "github.com/authzed/controller-idioms/state/middleware"
)

pipeline := state.Sequence(
    authenticate,
    authorize,
    process,
)

// Apply explicit transformations
logged := state.WithMiddleware(
    pipeline,
    middleware.RecursiveLogging(logger),
    middleware.RecursiveMetrics(collector),
)

state.Run(ctx, logged)
```

### Example 2: Context-Based Dynamic Transforms

```go
import (
    "github.com/authzed/controller-idioms/state"
    "github.com/authzed/controller-idioms/state/transform"
)

pipeline := state.Sequence(
    authenticate,
    authorize,
    process,
)

// Add transforms via context
ctx = transform.WithLogging(ctx, logger)
ctx = transform.WithMetrics(ctx, collector)

if debugMode {
    ctx = transform.WithTracing(ctx, tracer)
}

// Automatically applies all registered transforms
transform.RunWithTransforms(ctx, pipeline)
```

### Example 3: Property Verification

```go
import (
    "github.com/authzed/controller-idioms/state"
    "github.com/authzed/controller-idioms/state/verify"
)

pipeline := state.Sequence(
    step1,
    step2,
    step3,
)

// Verify safety properties
if err := verify.VerifyNoPanic(pipeline); err != nil {
    t.Errorf("Pipeline panics: %v", err)
}

// Verify liveness properties
if err := verify.VerifyEventuallyTerminates(pipeline, 5*time.Second); err != nil {
    t.Errorf("Pipeline does not terminate: %v", err)
}

// Verify temporal properties
if err := verify.VerifyOrdering(pipeline, "authenticate", "authorize"); err != nil {
    t.Errorf("Invalid ordering: %v", err)
}
```

### Example 4: Controller Integration

```go
func (c *Controller) Reconcile(ctx context.Context, req Request) error {
    // Build pipeline
    pipeline := state.Sequence(
        c.validate,
        c.process,
        c.cleanup,
    )
    
    // Add transforms based on config
    if c.config.EnableLogging {
        ctx = transform.WithLogging(ctx, c.logger)
    }
    if c.config.EnableMetrics {
        ctx = transform.WithMetrics(ctx, c.metrics)
    }
    if c.config.EnableCircuitBreaker {
        ctx = transform.WithCircuitBreaker(ctx, 5)
    }
    
    // Run with all transforms applied
    transform.RunWithTransforms(ctx, pipeline)
    
    return ctx.Err()
}
```

## Implementation Phases

### Phase 1: Core Infrastructure
**Goal:** Enable recursive middleware pattern

**Deliverables:**
- `state/transform/transform.go` - RecursiveMiddleware helper, wrapStep utility
- `state/transform/context.go` - Context-based transform registry
- `state/transform/run.go` - RunWithTransforms implementation
- `state/transform/transform_test.go` - Unit tests for core infrastructure

**Success Criteria:**
- Can write recursive middleware that propagates through continuations
- Can register transforms in context
- RunWithTransforms applies all registered transforms correctly

### Phase 2: Built-in Middleware Library
**Goal:** Provide common recursive middleware implementations

**Deliverables:**
- `state/middleware/logging.go` - RecursiveLogging
- `state/middleware/metrics.go` - RecursiveMetrics
- `state/middleware/tracing.go` - RecursiveTracing (OpenTelemetry)
- `state/middleware/circuitbreaker.go` - RecursiveCircuitBreaker
- `state/middleware/ratelimit.go` - RecursiveRateLimit
- `state/middleware/middleware_test.go` - Integration tests

**Success Criteria:**
- Each middleware works correctly with simple pipelines
- Middleware composes correctly (multiple can be applied)
- Proper thread-safety for concurrent execution
- OpenTelemetry integration produces valid traces

### Phase 3: Model Checking
**Goal:** Enable property verification for pipelines

**Deliverables:**
- `state/verify/trace.go` - ExecutionTrace infrastructure
- `state/verify/verify.go` - Property verification functions
- `state/verify/verify_test.go` - Verification test suite

**Success Criteria:**
- Can verify safety properties (no panic, invariants)
- Can verify liveness properties (termination, progress)
- Can verify temporal properties (ordering)
- ExecutionTrace captures complete execution path

### Phase 4: Code Generation (Future Work)
**Goal:** Static analysis and compile-time verification

**Deferred to separate design doc.** Key ideas:
- Parse pipeline construction code
- Build static graph representation  
- Generate property-based tests
- Exhaustive path exploration for complex branching
- Tool with `+pipeline:` annotations (kubebuilder-style)

**Rationale for deferral:**
- Runtime tracing provides 80% of value
- Code generation adds complexity
- Can be added later without breaking changes
- Needs separate design discussion

## Key Benefits

### Developer Productivity
- Add logging/metrics/tracing with one line
- No modification to business logic required
- Composable cross-cutting concerns
- Clean separation between pipeline logic and instrumentation

### Runtime Flexibility
- Toggle behaviors via context flags
- A/B testing and feature flags
- Debug modes in production without redeployment
- Circuit breakers and rate limiting without code changes

### Testing & Verification
- Property-based testing for pipelines
- Automated verification of safety/liveness properties
- Execution replay for debugging
- Trace analysis for performance optimization

## Design Decisions

### Why Separate Packages?

**Decision:** Keep `state` pure, provide extensions in `state/transform`, `state/middleware`, `state/verify`

**Rationale:**
- `state` remains minimal and dependency-free
- Users can opt-in to only what they need
- Clear separation of concerns
- Easier to maintain and test
- Extensions don't pollute core API

**Alternative considered:** Put everything in `state` package
- Rejected: Would add many dependencies (OpenTelemetry, metrics libraries, etc.)
- Rejected: Would make `state` harder to understand and maintain

### Why Runtime Tracing Instead of Static Analysis?

**Decision:** Implement runtime tracing first, defer code generation

**Rationale:**
- Runtime tracing is simpler to implement
- Works with any pipeline construction pattern
- Provides immediate value for debugging
- Recursive middleware enables most use cases
- Code generation can be added later without breaking changes

**Alternative considered:** Start with code generation
- Rejected: More complex to implement correctly
- Rejected: Requires parser, AST manipulation, test generation
- Rejected: Go doesn't have macros, so code gen is the only option for compile-time analysis
- Deferred: Will revisit in Phase 4 once runtime approach is proven

### Why Context-Based Transform Registry?

**Decision:** Allow transforms to be registered in context for automatic application

**Rationale:**
- Aligns with Go idioms (context for request-scoped data)
- Enables dynamic configuration without changing pipeline code
- Supports feature flags and debug modes naturally
- Clean separation between pipeline definition and runtime behavior

**Alternative considered:** Global registry
- Rejected: Not thread-safe without locks
- Rejected: Makes testing difficult (global state)
- Rejected: Doesn't support per-request configuration

## Open Questions

None at this time. Design is complete and ready for implementation.

## Future Extensions

### Code Generation Tool
- Static pipeline analysis
- Compile-time property verification
- Generated property-based tests
- Exhaustive path exploration

### Additional Middleware
- Retry with exponential backoff
- Timeout enforcement at each step
- Request deduplication
- Caching layer
- A/B testing framework

### Advanced Verification
- Deadlock detection
- Resource leak detection
- Performance regression detection
- Concurrency property verification

## References

- `state/state.go` - Core monadic pipeline implementation
- `state/formal_test.go` - Category theory validation
- `queue/handlers.go` - v1 middleware pattern (inspiration)
- OpenTelemetry Go SDK - Tracing integration
- Property-based testing literature - Verification approach
