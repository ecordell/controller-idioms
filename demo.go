// Package demo demonstrates the final naming scheme for the state package.
// This showcases Handler/NewStep/Step() with perfect context threading.
package main

import (
	"context"
	"fmt"

	"github.com/authzed/controller-idioms/state"
)

func main() {
	fmt.Println("=== State Package Final Naming Scheme Demo ===")
	fmt.Println()

	demonstrateNaming()
	fmt.Println()
	demonstrateContextThreading()
	fmt.Println()
	demonstrateComposition()
	fmt.Println()
	demonstrateBranching()
	fmt.Println()
	fmt.Println("✅ All demos completed successfully!")
}

func demonstrateNaming() {
	fmt.Println("🏷️  Final Naming Scheme:")
	fmt.Println("   • Handler interface with Handle(context.Context) Handler method")
	fmt.Println("   • NewHandler constructor type: func(next Handler) Handler")
	fmt.Println("   • Step() lifting function: transforms pure functions into handlers")
	fmt.Println()

	ctx := context.Background()

	// Step() creates a NewStep from a pure function
	authenticate := state.Step(func(ctx context.Context) context.Context {
		fmt.Println("   📝 Step() creates composable handlers from pure functions")
		return context.WithValue(ctx, "user", "demo-user")
	})

	// NewHandler is the constructor type - can be stored and composed
	var constructor state.NewHandler = authenticate
	fmt.Println("   🏗️  NewHandler is the constructor type for building pipelines")

	// Handler is the execution interface with Handle() method
	_ = constructor.Handler() // Handler implements the execution interface
	fmt.Println("   ⚡ Handler is the execution interface with Handle() method")

	// Run executes the pipeline
	state.Run(ctx, authenticate)
	fmt.Println("   🚀 Run() executes NewStep pipelines")
}

func demonstrateContextThreading() {
	fmt.Println("🔄 Context Threading - The Core Innovation:")
	ctx := context.Background()

	pipeline := state.Sequence(
		state.Step(func(ctx context.Context) context.Context {
			fmt.Println("   Step 1: Adding 'service' to context")
			return context.WithValue(ctx, "service", "auth-service")
		}),
		state.Step(func(ctx context.Context) context.Context {
			service := ctx.Value("service").(string)
			fmt.Printf("   Step 2: Found service '%s', adding version\n", service)
			return context.WithValue(ctx, "version", "v1.2.3")
		}),
		state.Step(func(ctx context.Context) context.Context {
			service := ctx.Value("service").(string)
			version := ctx.Value("version").(string)
			fmt.Printf("   Step 3: Processing %s:%s\n", service, version)
			return context.WithValue(ctx, "processed", true)
		}),
		state.Step(func(ctx context.Context) context.Context {
			if processed := ctx.Value("processed").(bool); processed {
				fmt.Println("   ✅ Context threading successful - all values preserved!")
			}
			return ctx
		}),
	)

	state.Run(ctx, pipeline)
}

func demonstrateComposition() {
	fmt.Println("🧩 Composition Operators:")
	ctx := context.Background()

	// Sequential composition
	setup := state.Step(func(ctx context.Context) context.Context {
		fmt.Println("   🔧 Sequential: Setting up resources")
		return context.WithValue(ctx, "ready", true)
	})

	// Parallel composition
	monitoring := state.Parallel(
		state.Step(func(ctx context.Context) context.Context {
			fmt.Println("   📊 Parallel: Starting metrics collector")
			return ctx
		}),
		state.Step(func(ctx context.Context) context.Context {
			fmt.Println("   🔍 Parallel: Starting health checker")
			return ctx
		}),
		state.Step(func(ctx context.Context) context.Context {
			fmt.Println("   📝 Parallel: Starting audit logger")
			return ctx
		}),
	)

	cleanup := state.Step(func(ctx context.Context) context.Context {
		fmt.Println("   🧹 Sequential: All monitoring started, continuing...")
		return ctx
	})

	pipeline := state.Sequence(setup, monitoring, cleanup)
	state.Run(ctx, pipeline)
}

func demonstrateBranching() {
	fmt.Println("🌿 Branching and Decision Making:")

	// Test different scenarios
	scenarios := []struct {
		name string
		ctx  context.Context
	}{
		{"Admin User", context.WithValue(context.Background(), "role", "admin")},
		{"Regular User", context.WithValue(context.Background(), "role", "user")},
		{"Guest User", context.WithValue(context.Background(), "role", "guest")},
	}

	for _, scenario := range scenarios {
		fmt.Printf("   Testing: %s\n", scenario.name)

		pipeline := state.Sequence(
			state.Step(func(ctx context.Context) context.Context {
				role := ctx.Value("role").(string)
				fmt.Printf("     🔐 Authenticating %s\n", role)
				return context.WithValue(ctx, "authenticated", true)
			}),

			// Multi-way branching with Enum
			state.Enum(
				func(ctx context.Context) string {
					return ctx.Value("role").(string)
				},
				map[string]state.NewHandler{
					"admin": state.Step(func(ctx context.Context) context.Context {
						fmt.Println("     👑 Admin access: Full permissions granted")
						return context.WithValue(ctx, "permissions", "all")
					}),
					"user": state.Step(func(ctx context.Context) context.Context {
						fmt.Println("     👤 User access: Standard permissions granted")
						return context.WithValue(ctx, "permissions", "standard")
					}),
				},
				// Default case
				state.Step(func(ctx context.Context) context.Context {
					fmt.Println("     🚪 Guest access: Limited permissions granted")
					return context.WithValue(ctx, "permissions", "read-only")
				}),
			),

			state.Step(func(ctx context.Context) context.Context {
				permissions := ctx.Value("permissions").(string)
				fmt.Printf("     ✅ Access configured with %s permissions\n", permissions)
				return ctx
			}),
		)

		state.Run(scenario.ctx, pipeline)
		fmt.Println()
	}
}
