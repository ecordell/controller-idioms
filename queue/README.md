# Queue Package

The `queue` package provides helpers for working with client-go's `workqueues` and integrates seamlessly with the `state` package to provide clean control flow patterns for Kubernetes controllers.

## Overview

This package provides two main categories of functionality:

1. **Queue Operations** (`controls.go`) - Direct queue control operations
2. **Queue Handlers** (`handlers.go`) - State package integration for clean control flow

## Queue Operations

The traditional queue operations allow you to control queue behavior from within handlers:

```go
import "github.com/authzed/controller-idioms/queue"

func myHandler(ctx context.Context) {
    queueOps := queue.NewQueueOperationsCtx()
    
    if someCondition {
        queueOps.Done(ctx)
        return
    }
    
    if needsRetry {
        queueOps.RequeueAfter(ctx, 30*time.Second)
        return
    }
}
```

Available operations:
- `Done(ctx)` - Mark processing complete
- `Requeue(ctx)` - Requeue immediately  
- `RequeueAfter(ctx, duration)` - Requeue after delay
- `RequeueErr(ctx, err)` - Requeue with error
- `RequeueAPIErr(ctx, err)` - Requeue with API error handling

## Queue Handlers (New!)

The queue handlers provide clean integration with the `state` package, allowing you to express controller logic as composable pipelines with built-in queue control:

### Basic Handlers

Instead of manually calling queue operations and returning, you can now use:

```go
import (
    "github.com/authzed/controller-idioms/queue"
    "github.com/authzed/controller-idioms/state"
)

// Old way
func oldHandler(ctx context.Context) {
    // do work...
    if success {
        queue.NewQueueOperationsCtx().Done(ctx)
        return
    }
    queue.NewQueueOperationsCtx().RequeueAfter(ctx, 5*time.Second)
}

// New way
pipeline := state.Sequence(
    state.Action(func(ctx context.Context) {
        // do work...
    }),
    state.Decision(
        func(ctx context.Context) bool { return success },
        queue.Done(),                    // Success path
        queue.RequeueAfter(5*time.Second), // Retry path
    ),
)
```

### Available Queue Handlers

#### Terminating Handlers
These handlers terminate the pipeline by calling queue operations:

- `queue.Done()` - Mark processing complete and stop
- `queue.Requeue()` - Requeue immediately and stop
- `queue.RequeueAfter(duration)` - Requeue after delay and stop
- `queue.RequeueErr(err)` - Requeue with error and stop
- `queue.RequeueAPIErr(err)` - Requeue with API error handling and stop

#### Conditional Handlers
These handlers conditionally call queue operations or continue:

- `queue.ConditionalRequeue(condition)` - Requeue if condition is true, otherwise continue
- `queue.ConditionalRequeueAfter(condition, duration)` - Requeue after delay if condition is true, otherwise continue  
- `queue.ConditionalDone(condition)` - Mark done if condition is true, otherwise continue

#### Control Flow Handlers

- `queue.OnError(errorHandler, successHandler)` - Choose handler based on context error state

### Real-World Controller Examples

#### Resource Readiness Check

```go
controllerPipeline := state.Sequence(
    // Set up finalizer
    state.Action(func(ctx context.Context) {
        setFinalizer(ctx)
    }),
    
    // Wait for dependencies to be ready
    queue.ConditionalRequeueAfter(
        func(ctx context.Context) bool {
            return !dependenciesReady(ctx)
        },
        2*time.Minute, // Check again in 2 minutes
    ),
    
    // Process the resource
    state.Action(func(ctx context.Context) {
        processResource(ctx)
    }),
    
    // Handle any processing errors
    queue.OnError(
        queue.RequeueErr(fmt.Errorf("processing failed")),
        queue.Done(), // Success
    ),
)

state.Run(ctx, controllerPipeline)
```

#### Multi-Phase Controller

```go
controllerPipeline := state.Sequence(
    // Phase 1: Validation
    state.Action(func(ctx context.Context) {
        validateResource(ctx)
    }),
    
    // Stop if validation failed
    queue.ConditionalRequeueErr(
        func(ctx context.Context) bool { return ctx.Err() != nil },
        fmt.Errorf("validation failed"),
    ),
    
    // Phase 2: Resource creation
    state.Decision(
        func(ctx context.Context) bool {
            return resourceExists(ctx)
        },
        // Resource exists - update it
        state.Sequence(
            state.Action(func(ctx context.Context) {
                updateResource(ctx)
            }),
            queue.Done(),
        ),
        // Resource doesn't exist - create it
        state.Sequence(
            state.Action(func(ctx context.Context) {
                createResource(ctx)
            }),
            // Wait for resource to be ready after creation
            queue.RequeueAfter(10*time.Second),
        ),
    ),
)
```

#### Error Handling with Retry Logic

```go
resilientPipeline := state.Sequence(
    // Attempt risky operation
    state.Action(func(ctx context.Context) {
        err := riskyKubernetesAPICall(ctx)
        if err != nil {
            // Store error in context for later handling
            storeError(ctx, err)
        }
    }),
    
    // Handle different types of errors appropriately
    state.Switch(
        func(ctx context.Context) string {
            if err := getStoredError(ctx); err != nil {
                if apierrors.IsNotFound(err) {
                    return "not-found"
                } else if apierrors.IsConflict(err) {
                    return "conflict"
                } else if apierrors.IsServiceUnavailable(err) {
                    return "service-unavailable"
                }
                return "unknown-error"
            }
            return "success"
        },
        map[string]state.NewHandler{
            "not-found": queue.RequeueAfter(30*time.Second),
            "conflict": queue.Requeue(), // Retry immediately
            "service-unavailable": queue.RequeueAfter(5*time.Minute),
            "unknown-error": queue.RequeueErr(getStoredError(ctx)),
            "success": queue.Done(),
        },
        queue.RequeueErr(fmt.Errorf("unexpected error state")),
    ),
)
```

### Benefits

#### Clean Control Flow
Before:
```go
func complexHandler(ctx context.Context) {
    validateResource(ctx)
    if ctx.Err() != nil {
        queue.NewQueueOperationsCtx().RequeueErr(ctx, ctx.Err())
        return
    }
    
    if !dependenciesReady(ctx) {
        queue.NewQueueOperationsCtx().RequeueAfter(ctx, 2*time.Minute)
        return
    }
    
    processResource(ctx)
    if ctx.Err() != nil {
        queue.NewQueueOperationsCtx().RequeueErr(ctx, ctx.Err())
        return  
    }
    
    queue.NewQueueOperationsCtx().Done(ctx)
}
```

After:
```go
pipeline := state.Sequence(
    validateResource,
    queue.ConditionalRequeueAfter(
        func(ctx context.Context) bool { return !dependenciesReady(ctx) },
        2*time.Minute,
    ),
    processResource,
    queue.OnError(
        queue.RequeueErr(ctx.Err()),
        queue.Done(),
    ),
)
```

#### Composable and Testable
- Each handler can be tested independently
- Complex logic can be built from simple, reusable components
- Clear separation between business logic and queue control

#### Type Safety
- All handlers are statically typed
- Compile-time guarantees about pipeline composition
- No runtime handler lookup by string IDs

## Migration Guide

### From Direct Queue Operations

```go
// Before
func handler(ctx context.Context) {
    doWork(ctx)
    if success {
        queue.NewQueueOperationsCtx().Done(ctx)
        return
    }
    queue.NewQueueOperationsCtx().RequeueAfter(ctx, 30*time.Second)
    return
}

// After  
pipeline := state.Sequence(
    state.Action(doWork),
    state.Decision(
        func(ctx context.Context) bool { return success },
        queue.Done(),
        queue.RequeueAfter(30*time.Second),
    ),
)
state.Run(ctx, pipeline)
```

### From Handler Package Patterns

The queue handlers work seamlessly with the state package's replacement of the handler package patterns. See the [state package documentation](../state/README.md) for more details on migrating from the handler package.

## Integration with State Package

Queue handlers are designed to work naturally with all state package operators:

- Use with `state.Sequence` for linear processing
- Use with `state.Decision` for conditional logic  
- Use with `state.Enum` and `state.Switch` for multi-way branching
- Use with `state.Parallel` for concurrent operations
- Compose with any other `state.NewHandler` functions

This provides unprecedented flexibility in expressing complex controller logic while maintaining clean separation between business logic and queue management.