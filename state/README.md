# state

The `state` package provides a continuation-passing style framework for building composable state machines and processing pipelines in Kubernetes controllers.

## Overview

The core abstraction is a `Handler`, which represents a single handler in a computation that returns the next handler to execute, or `nil` to terminate:

```go
type Handler interface {
    Handle(context.Context) Handler
}
```

This continuation-passing style eliminates the need for complex builder patterns and ID-based handler lookup, making composition more direct and intuitive.

## Key Concepts

### Handler
A `Handler` represents a single handler in a processing pipeline. Each handler processes a context and returns the next handler to execute, or `nil` to terminate the pipeline.

### NewHandler
A `NewHandler` is a factory function that creates a `Handler` when called. This is the basic building block for composing handler pipelines:

```go
type NewHandler func(next Handler) Handler
```

### Composition Operators
- **Sequence**: Executes handlers sequentially
- **Parallel**: Executes handlers concurrently
- **Decision**: Conditional branching based on context
- **Enum**: Multi-way branching based on enum values
- **Switch**: Multi-way branching based on string values
- **Map**: Transform context between handlers
- **Bind**: Monadic composition for dependent handlers
- **CallAndCheck**: Execute a handler and inspect results before continuing
- **Reuse**: Reuse handlers with custom logic based on execution results

## Basic Usage

### Simple Sequential Pipeline

```go
pipeline := state.Sequence(
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("first handler")
        return ctx
    }),
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("second handler")
        return ctx
    }),
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("third handler")
        return ctx
    }),
)

state.Run(ctx, pipeline)
```

### Conditional Branching

```go
pipeline := state.Sequence(
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("initialization")
        return ctx
    }),
    state.Decision(
        func(ctx context.Context) bool {
            return someCondition(ctx)
        },
        // True branch
        state.Step(func(ctx context.Context) context.Context {
            fmt.Println("condition was true")
            return ctx
        }),
        // False branch  
        state.Step(func(ctx context.Context) context.Context {
            fmt.Println("condition was false")
            return ctx
        }),
    ),
)
```

### Parallel Execution

```go
pipeline := state.Sequence(
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("before parallel work")
        return ctx
    }),
    state.Parallel(
        state.Step(func(ctx context.Context) context.Context {
            fmt.Println("parallel task 1")
            return ctx
        }),
        state.Step(func(ctx context.Context) context.Context {
            fmt.Println("parallel task 2") 
            return ctx
        }),
        state.Step(func(ctx context.Context) context.Context {
            fmt.Println("parallel task 3")
            return ctx
        }),
    ),
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("after parallel work")
        return ctx
    }),
)
```

### Multi-way Branching with Enum

```go
pipeline := state.Enum(
    func(ctx context.Context) string {
        return getResourceType(ctx)
    },
    map[string]state.NewHandler{
        "deployment": state.Step(func(ctx context.Context) context.Context {
            fmt.Println("handling deployment")
            return ctx
        }),
        "service": state.Step(func(ctx context.Context) context.Context {
            fmt.Println("handling service")
            return ctx
        }),
        "configmap": state.Step(func(ctx context.Context) context.Context {
            fmt.Println("handling configmap")
            return ctx
        }),
    },
    // Default case
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("unknown resource type")
        return ctx
    }),
)
```

### String-based Branching with Switch

```go
pipeline := state.Switch(
    func(ctx context.Context) string {
        return getOperationPhase(ctx)
    },
    map[string]state.NewHandler{
        "pending":   initializationStep,
        "running":   monitoringStep,
        "completed": cleanupStep,
        "failed":    recoveryStep,
    },
    unknownPhaseStep, // Default case
)
```

### Handler Reuse and Wrapping

The state package provides powerful patterns for reusing handlers and wrapping them with additional logic, similar to the wrapping patterns from the handler package:

```go
// Reuse a handler with custom logic after execution
pipeline := state.Reuse(
    validationHandler,
    func(ctx context.Context) state.NewHandler {
        if ctx.Err() != nil {
            return errorHandlingHandler
        }
        return successHandler
    },
)

// Call a handler and check results before continuing
pipeline := state.CallAndCheck(
    riskyOperation,
    func(ctx context.Context, result state.Handler) state.Handler {
        if ctx.Err() != nil {
            return recoveryHandler.Handler()
        }
        return cleanupHandler.Handler()
    },
)

// Conditional continuation based on execution results
pipeline := state.CallAndContinueIf(
    validationHandler,
    func(ctx context.Context) bool {
        return ctx.Err() == nil // Continue if no error
    },
    successPath,    // Continue here if condition is true
    errorPath,      // Go here if condition is false
)
```

This replicates the old handler pattern:
```go
// Old handler pattern
if len(validations) == 0 {
    h.Next.Handle(ctx)
    return
}
h.ensureValidatingAdmissionPolicy(ctx)
if errors.Is(ctx.Err(), context.Canceled) {
    return
}

// New state pattern  
state.Reuse(
    state.Action(func(ctx context.Context) {
        if len(validations) == 0 {
            return // Skip to after-execution logic
        }
        ensureValidatingAdmissionPolicy(ctx)
    }),
    func(ctx context.Context) state.NewHandler {
        if errors.Is(ctx.Err(), context.Canceled) {
            return nil // Stop execution
        }
        return nextHandler
    },
)
```

## Monadic Operations

The state package provides monadic operations for advanced composition:

### Map - Transform Context

```go
pipeline := state.Map(
    func(ctx context.Context) context.Context {
        // Transform the context
        return context.WithValue(ctx, "key", "transformed_value")
    },
    state.Step(func(ctx context.Context) context.Context {
        value := ctx.Value("key").(string)
        fmt.Printf("received: %s\n", value)
        return ctx
    }),
)
```

### Bind - Dependent Composition

```go
pipeline := state.Bind(
    state.Step(func(ctx context.Context) context.Context {
        fmt.Println("first operation")
        return ctx
    }),
    func() state.NewHandler {
        return state.Step(func(ctx context.Context) context.Context {
            fmt.Println("dependent operation")
            return ctx
        })
    },
)
```

// Main controller pipeline
mainPipeline := state.Sequence(
    state.Step(func(ctx context.Context) context.Context {
        // Set finalizer
        return ctx
    }),
    state.Step(func(ctx context.Context) context.Context {
        // Check for safe deletion
        return ctx
    }),
    state.Step(func(ctx context.Context) context.Context {
        // Check pause condition
        return ctx
    }),
    resourceHandler, // Use the resource handler defined above
)

// Execute the pipeline
state.Run(ctx, mainPipeline)
```

## Advantages Over Builder Pattern

### Before (handler package):
```go
// Complex builder pattern with IDs
hasSecretHandler := handler.Chain(
    ensureMetadataBuilder,
    ensureScopedCredsBuilder,
).Handler("hasSecret")

directSecretHandler := handler.Chain(
    adoptSecretBuilder,
    hasSecretHandler.Builder(),
).Handler("directSecret")

mainHandler := handler.Chain(
    setFinalizerBuilder,
    validateInstanceBuilder(hasSecretHandler, directSecretHandler),
).Handler("main")
```

### After (state package):
```go
// Direct composition without IDs
hasSecretChain := state.Sequence(
    ensureMetadata,
    ensureScopedCreds,
)

mainPipeline := state.Sequence(
    setFinalizer,
    state.Switch(
        getInstanceType,
        map[string]state.NewHandler{
            "database": hasSecretChain,
            "legacy":   state.Sequence(adoptSecret, hasSecretChain),
            "external": externalSecretChain,
        },
        defaultSecretChain, // Default case
    ),
)
```

## Key Benefits

1. **No ID Management**: Direct references eliminate the need for handler IDs and lookup
2. **Simpler Composition**: Handlers compose naturally without complex builder patterns  
3. **Type Safety**: Factory functions provide compile-time safety
4. **Multi-way Branching**: Enum and Switch operators handle complex branching logic elegantly
5. **Monadic Operations**: Support for advanced functional composition patterns
6. **Clear Control Flow**: Continuation-passing makes execution flow explicit
7. **Handler Reuse**: Call and reuse handlers with custom logic based on execution results
8. **Reusability**: Handlers can be easily reused across different pipelines
9. **Testability**: Individual handlers and compositions are easy to test

## Integration with Queue Operations

The state package works seamlessly with the existing `queue` operations for controller lifecycle management:

```go
import "github.com/authzed/controller-idioms/queue"

validationHandler := state.Action(func(ctx context.Context) {
    if err := validateResource(ctx); err != nil {
        queue.NewQueueOperationsCtx().RequeueErr(ctx, err)
        return
    }
    // Continue processing...
})
```

The continuation-passing style provides a clean, functional approach to building complex controller state machines while maintaining compatibility with the existing controller-idioms ecosystem.