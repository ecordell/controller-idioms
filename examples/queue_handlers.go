package main

import (
	"context"
	"fmt"
	"time"

	"github.com/authzed/controller-idioms/queue"
	"github.com/authzed/controller-idioms/queue/fake"
	"github.com/authzed/controller-idioms/state"
)

// ExampleController demonstrates how to use queue handlers with the state package
// to build clean, composable controller logic.
func ExampleController() {
	ctx := context.Background()

	// Set up fake queue for demonstration
	fakeQueue := &fake.FakeInterface{}
	queueCtx := queue.NewQueueOperationsCtx()
	ctx = queueCtx.WithValue(ctx, fakeQueue)

	// Simulate controller context with resource state
	resourceCtx := context.WithValue(ctx, "resource-ready", false)
	resourceCtx = context.WithValue(resourceCtx, "validation-passed", true)
	resourceCtx = context.WithValue(resourceCtx, "dependencies-ready", true)

	// Build controller pipeline using queue handlers
	controllerPipeline := state.Sequence(
		// Step 1: Set finalizer
		state.Action(func(ctx context.Context) {
			fmt.Println("✓ Setting finalizer")
		}),

		// Step 2: Validate resource specification
		state.Action(func(ctx context.Context) {
			fmt.Println("✓ Validating resource specification")
			// In real code: validateResourceSpec(ctx)
		}),

		// Step 3: Stop if validation failed
		state.Decision(
			func(ctx context.Context) bool {
				return !ctx.Value("validation-passed").(bool)
			},
			queue.RequeueErr(fmt.Errorf("resource validation failed")),
			state.Action(func(ctx context.Context) {
				// Continue if validation passed
			}),
		),

		// Step 4: Wait for dependencies
		queue.ConditionalRequeueAfter(
			func(ctx context.Context) bool {
				ready := ctx.Value("dependencies-ready").(bool)
				if !ready {
					fmt.Println("⏳ Dependencies not ready, will retry in 30s")
				}
				return !ready
			},
			30*time.Second,
		),

		// Step 5: Check if resource needs creation or update
		state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("resource-ready").(bool)
			},
			// Resource exists - update path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("✓ Resource exists, updating...")
				}),
				state.Action(func(ctx context.Context) {
					fmt.Println("✓ Resource updated successfully")
				}),
				queue.Done(),
			),
			// Resource doesn't exist - creation path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("✓ Creating new resource...")
				}),
				state.Action(func(ctx context.Context) {
					fmt.Println("✓ Resource created, waiting for readiness")
				}),
				// Requeue to check if resource is ready
				queue.RequeueAfter(10*time.Second),
			),
		),
	)

	// Execute the controller pipeline
	fmt.Println("🚀 Starting controller execution...")
	state.Run(resourceCtx, controllerPipeline)
	fmt.Println("✅ Controller execution completed")

	// Show what queue operations were called
	fmt.Printf("Queue operations: RequeueAfter called %d times\n", fakeQueue.RequeueAfterCallCount())
}

// ExampleErrorHandling demonstrates error handling patterns with queue handlers
func ExampleErrorHandling() {
	ctx := context.Background()

	// Set up fake queue for demonstration
	fakeQueue := &fake.FakeInterface{}
	queueCtx := queue.NewQueueOperationsCtx()
	ctx = queueCtx.WithValue(ctx, fakeQueue)

	// Simulate different error conditions
	scenarios := []struct {
		name     string
		hasError bool
		errorMsg string
	}{
		{"successful-processing", false, ""},
		{"validation-error", true, "validation failed"},
		{"api-timeout", true, "request timeout"},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- Scenario: %s ---\n", scenario.name)

		// Create context with error if needed
		scenarioCtx := ctx
		if scenario.hasError {
			// Simulate error by canceling context
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			scenarioCtx = cancelCtx
		}

		pipeline := state.Sequence(
			state.Action(func(ctx context.Context) {
				fmt.Println("Processing resource...")
			}),

			// Handle errors gracefully
			queue.OnError(
				// Error path - different handling based on error type
				state.Switch(
					func(ctx context.Context) string {
						if scenario.errorMsg == "validation failed" {
							return "validation"
						} else if scenario.errorMsg == "request timeout" {
							return "timeout"
						}
						return "unknown"
					},
					map[string]state.NewHandler{
						"validation": state.Sequence(
							state.Action(func(ctx context.Context) {
								fmt.Println("❌ Validation error - immediate requeue")
							}),
							queue.Requeue(),
						),
						"timeout": state.Sequence(
							state.Action(func(ctx context.Context) {
								fmt.Println("⏳ Timeout error - retry after delay")
							}),
							queue.RequeueAfter(1*time.Minute),
						),
					},
					state.Sequence(
						state.Action(func(ctx context.Context) {
							fmt.Println("⚠️  Unknown error - requeue with error")
						}),
						queue.RequeueErr(fmt.Errorf("unknown error occurred")),
					),
				),
				// Success path
				state.Sequence(
					state.Action(func(ctx context.Context) {
						fmt.Println("✅ Processing completed successfully")
					}),
					queue.Done(),
				),
			),
		)

		state.Run(scenarioCtx, pipeline)
	}
}

// ExampleConditionalLogic demonstrates conditional queue operations
func ExampleConditionalLogic() {
	ctx := context.Background()

	// Set up fake queue for demonstration
	fakeQueue := &fake.FakeInterface{}
	queueCtx := queue.NewQueueOperationsCtx()
	ctx = queueCtx.WithValue(ctx, fakeQueue)

	scenarios := []struct {
		name             string
		resourceExists   bool
		resourceReady    bool
		processingNeeded bool
	}{
		{"new-resource", false, false, true},
		{"existing-not-ready", true, false, true},
		{"existing-ready", true, true, false},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- Scenario: %s ---\n", scenario.name)

		scenarioCtx := context.WithValue(ctx, "resource-exists", scenario.resourceExists)
		scenarioCtx = context.WithValue(scenarioCtx, "resource-ready", scenario.resourceReady)
		scenarioCtx = context.WithValue(scenarioCtx, "processing-needed", scenario.processingNeeded)

		pipeline := state.Sequence(
			// Check current state
			state.Action(func(ctx context.Context) {
				exists := ctx.Value("resource-exists").(bool)
				ready := ctx.Value("resource-ready").(bool)
				fmt.Printf("Resource exists: %v, ready: %v\n", exists, ready)
			}),

			// Conditional processing based on resource state
			queue.ConditionalRequeueAfter(
				func(ctx context.Context) bool {
					exists := ctx.Value("resource-exists").(bool)
					ready := ctx.Value("resource-ready").(bool)
					needsWait := exists && !ready

					if needsWait {
						fmt.Println("⏳ Resource exists but not ready, waiting...")
					}
					return needsWait
				},
				15*time.Second,
			),

			// Conditional processing
			queue.ConditionalDone(
				func(ctx context.Context) bool {
					processingNeeded := ctx.Value("processing-needed").(bool)
					if !processingNeeded {
						fmt.Println("✅ No processing needed, marking done")
						return true
					}
					return false
				},
			),

			// If we reach here, processing is needed
			state.Action(func(ctx context.Context) {
				exists := ctx.Value("resource-exists").(bool)
				if exists {
					fmt.Println("✓ Updating existing resource")
				} else {
					fmt.Println("✓ Creating new resource")
				}
			}),

			// Mark as done after processing
			queue.Done(),
		)

		state.Run(scenarioCtx, pipeline)
	}
}

// main demonstrates all the examples
func main() {
	fmt.Println("=== Queue Handlers Examples ===\n")

	fmt.Println("1. Basic Controller Flow:")
	ExampleController()

	fmt.Println("\n2. Error Handling Patterns:")
	ExampleErrorHandling()

	fmt.Println("\n3. Conditional Logic:")
	ExampleConditionalLogic()

	fmt.Println("\n=== Examples Complete ===")
}
