// Package state provides a continuation-passing style framework for building
// composable state machines and processing pipelines in Kubernetes controllers.
//
// The core abstraction is a Handler, which represents a single handler in a
// computation that uses continuation-passing style - each handler returns the
// next handler to execute, or nil to terminate the pipeline.
//
//	type Handler interface {
//		Handle(context.Context) Handler
//	}
//
// This eliminates the need for complex builder patterns and ID-based handler
// lookup found in the handler package, making composition more direct and
// intuitive.
//
// # Basic Usage
//
// Create handlers using Step for operations with context transformation:
//
//	handler := state.Step(func(ctx context.Context) context.Context {
//		fmt.Println("processing...")
//		return ctx
//	})
//
// Compose handlers using Sequence for sequential execution:
//
//	pipeline := state.Sequence(
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("first")
//			return ctx
//		}),
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("second")
//			return ctx
//		}),
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("third")
//			return ctx
//		}),
//	)
//
//	state.Run(ctx, pipeline)
//
// # Conditional Logic
//
// Use Decision for binary branching:
//
//	pipeline := state.Decision(
//		func(ctx context.Context) bool {
//			return someCondition(ctx)
//		},
//		// True branch
//		state.Action(func(ctx context.Context) {
//			fmt.Println("condition was true")
//		}),
//		// False branch
//		state.Action(func(ctx context.Context) {
//			fmt.Println("condition was false")
//		}),
//	)
//
// Use Enum for multi-way branching:
//
//	pipeline := state.Enum(
//		func(ctx context.Context) string {
//			return getResourceType(ctx)
//		},
//		map[string]state.NewStep{
//			"deployment": state.Step(func(ctx context.Context) context.Context {
//				fmt.Println("handling deployment")
//				return ctx
//			}),
//			"service": state.Step(func(ctx context.Context) context.Context {
//				fmt.Println("handling service")
//				return ctx
//			}),
//			"configmap": state.Step(func(ctx context.Context) context.Context {
//				fmt.Println("handling configmap")
//				return ctx
//			}),
//		},
//		// Default case
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("unknown resource type")
//			return ctx
//		}),
//	)
//
// Use Switch for string-based branching (convenience wrapper for Enum):
//
//	pipeline := state.Switch(
//		func(ctx context.Context) string {
//			return getPhase(ctx)
//		},
//		map[string]state.NewStep{
//			"pending":   initStep,
//			"running":   monitorStep,
//			"completed": cleanupStep,
//		},
//		unknownPhaseStep,
//	)
//
// # Parallel Execution
//
// Use Parallel for concurrent execution:
//
//	pipeline := state.Parallel(
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("task 1")
//			return ctx
//		}),
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("task 2")
//			return ctx
//		}),
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("task 3")
//			return ctx
//		}),
//	)
//
// # Monadic Operations
//
// The package provides monadic operations for advanced composition:
//
//	// Map - transform context
//	pipeline := state.Map(
//		func(ctx context.Context) context.Context {
//			return context.WithValue(ctx, "key", "value")
//		},
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println(ctx.Value("key"))
//			return ctx
//		}),
//	)
//
//	// Bind - dependent composition
//	pipeline := state.Bind(
//		state.Step(func(ctx context.Context) context.Context {
//			fmt.Println("first")
//			return ctx
//		}),
//		func() state.NewStep {
//			return state.Step(func(ctx context.Context) context.Context {
//				fmt.Println("second")
//				return ctx
//			})
//		},
//	)
//
// # Controller Example
//
// A typical controller pipeline using branching logic:
//
//	mainPipeline := state.Sequence(
//		state.Step(func(ctx context.Context) context.Context {
//			// Set finalizer
//			setFinalizer(ctx)
//			return ctx
//		}),
//		state.Step(func(ctx context.Context) context.Context {
//			// Check for safe deletion
//			checkSafeDeletion(ctx)
//			return ctx
//		}),
//		state.Decision(
//			func(ctx context.Context) bool {
//				return hasDatabaseInstance(ctx)
//			},
//			// New style: has database instance
//			state.Sequence(
//				ensureMetadata,
//				createScopedCredentials,
//			),
//			// Old style: direct secret reference
//			state.Sequence(
//				adoptExistingSecret,
//				ensureMetadata,
//			),
//		),
//	)
//
//	state.Run(ctx, mainPipeline)
//
// # Comparison with Handler Package
//
// The state package simplifies the patterns from the handler package:
//
//	// handler package (old)
//	hasSecretHandler := handler.Chain(
//		ensureMetadataBuilder,
//		ensureCredsBuilder,
//	).Handler("hasSecret")
//
//	mainHandler := handler.Chain(
//		setFinalizerBuilder,
//		validateInstanceBuilder(hasSecretHandler, directSecretHandler),
//	).Handler("main")
//
//	// state package (new)
//	mainPipeline := state.Sequence(
//		setFinalizer,
//		state.Decision(
//			hasDatabaseInstance,
//			state.Sequence(ensureMetadata, ensureCreds),
//			state.Sequence(adoptSecret, ensureMetadata),
//		),
//	)
//
// # Benefits
//
// - No ID management: Direct references eliminate handler IDs and lookup
// - Simpler composition: Handlers compose naturally without builder patterns
// - Type safety: Factory functions provide compile-time safety
// - Multi-way branching: Enum and Switch operators handle complex branching elegantly
// - Monadic operations: Advanced functional composition patterns
// - Clear control flow: Continuation-passing makes execution explicit
// - Reusability: Handlers easily reused across different pipelines
// - Testability: Individual handlers and compositions easy to test
//
// The continuation-passing style provides a clean, functional approach to
// building complex controller state machines while maintaining compatibility
// with the existing controller-idioms ecosystem.
package state
