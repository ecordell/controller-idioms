package inspector

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/authzed/controller-idioms/state"
)

func ExampleRealTimeInspection() {
	fmt.Println("=== Real-Time State Machine Inspector ===")

	// Create a representative controller pipeline
	controllerPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("🚀 Controller starting")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("resourceExists").(bool)
			},
			// Resource exists - update path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("✏️  Updating existing resource")
				}),
				state.Action(func(ctx context.Context) {
					fmt.Println("📊 Resource updated successfully")
				}),
			),
			// Resource doesn't exist - create path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("➕ Creating new resource")
				}),
				state.Parallel(
					state.Action(func(ctx context.Context) {
						fmt.Println("🔧 Setting up component A")
					}),
					state.Action(func(ctx context.Context) {
						fmt.Println("⚙️  Setting up component B")
					}),
				),
				state.Action(func(ctx context.Context) {
					fmt.Println("✅ Resource created successfully")
				}),
			),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("🎯 Controller finished")
		}),
	)

	// Wrap with real-time inspection
	inspector, instrumentedPipeline := WithRealTimeInspection("demo-controller", controllerPipeline)

	// Start the inspector server in the background
	go func() {
		if err := inspector.StartServer(":8080"); err != nil {
			log.Printf("Inspector server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n--- Live Inspector Started ---")
	fmt.Println("🌐 Visit http://localhost:8080 for real-time visualization")
	fmt.Println("📊 Stately Inspector will show live state transitions")

	// Execute the CREATE path
	fmt.Println("\n--- Executing CREATE Path ---")
	createCtx := context.WithValue(context.Background(), "resourceExists", false)
	state.RunNewStage(createCtx, instrumentedPipeline)

	// Small delay to see the transition in the UI
	time.Sleep(500 * time.Millisecond)

	// Execute the UPDATE path
	fmt.Println("\n--- Executing UPDATE Path ---")
	updateCtx := context.WithValue(context.Background(), "resourceExists", true)
	state.RunNewStage(updateCtx, instrumentedPipeline)

	fmt.Println("\n✨ Real-time visualization complete!")
	fmt.Println("💡 The Stately Inspector shows:")
	fmt.Println("   - Live state transitions")
	fmt.Println("   - Current machine state")
	fmt.Println("   - Event history")
	fmt.Println("   - Context data")

	// Output:
	// === Real-Time State Machine Inspector ===
	//
	// --- Live Inspector Started ---
	// 🌐 Visit http://localhost:8080 for real-time visualization
	// 📊 Stately Inspector will show live state transitions
	//
	// --- Executing CREATE Path ---
	// 🚀 Controller starting
	// ➕ Creating new resource
	// 🔧 Setting up component A
	// ⚙️  Setting up component B
	// ✅ Resource created successfully
	// 🎯 Controller finished
	//
	// --- Executing UPDATE Path ---
	// 🚀 Controller starting
	// ✏️  Updating existing resource
	// 📊 Resource updated successfully
	// 🎯 Controller finished
	//
	// ✨ Real-time visualization complete!
	// 💡 The Stately Inspector shows:
	//    - Live state transitions
	//    - Current machine state
	//    - Event history
	//    - Context data
}

func ExampleIntegratingWithExistingController() {
	fmt.Println("=== Integrating Real-Time Inspector with Existing Controller ===")

	// This could be your existing controller reconcile function
	reconcileWorkflow := func() state.NewStage {
		return state.Sequence(
			state.Action(func(ctx context.Context) {
				fmt.Println("🔍 Validating resource spec")
			}),
			state.Switch(
				func(ctx context.Context) string {
					return ctx.Value("operation").(string)
				},
				map[string]state.NewStage{
					"create": state.Action(func(ctx context.Context) {
						fmt.Println("📦 Creating Kubernetes resources")
					}),
					"update": state.Action(func(ctx context.Context) {
						fmt.Println("🔄 Updating Kubernetes resources")
					}),
					"delete": state.Sequence(
						state.Action(func(ctx context.Context) {
							fmt.Println("🗑️  Deleting resources")
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("🧹 Cleaning up finalizers")
						}),
					),
				},
				state.Action(func(ctx context.Context) {
					fmt.Println("❌ Unknown operation")
				}),
			),
			state.Action(func(ctx context.Context) {
				fmt.Println("📈 Updating status")
			}),
		)
	}

	// Add real-time inspection with ONE LINE
	inspector, inspectedWorkflow := WithRealTimeInspection("k8s-controller", reconcileWorkflow())

	// Start the inspector (typically in your main function)
	go inspector.StartServer(":8081")

	fmt.Println("\n🚀 Controller with live inspection running!")
	fmt.Println("📊 Inspector available at: http://localhost:8081")

	// Simulate controller operations
	operations := []string{"create", "update", "delete"}
	for _, op := range operations {
		fmt.Printf("\n--- Processing %s operation ---\n", op)
		ctx := context.WithValue(context.Background(), "operation", op)
		state.RunNewStage(ctx, inspectedWorkflow)

		// Brief pause to see transitions in UI
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n🎉 All operations visualized in real-time!")

	// Output:
	// === Integrating Real-Time Inspector with Existing Controller ===
	//
	// 🚀 Controller with live inspection running!
	// 📊 Inspector available at: http://localhost:8081
	//
	// --- Processing create operation ---
	// 🔍 Validating resource spec
	// 📦 Creating Kubernetes resources
	// 📈 Updating status
	//
	// --- Processing update operation ---
	// 🔍 Validating resource spec
	// 🔄 Updating Kubernetes resources
	// 📈 Updating status
	//
	// --- Processing delete operation ---
	// 🔍 Validating resource spec
	// 🗑️  Deleting resources
	// 🧹 Cleaning up finalizers
	// 📈 Updating status
	//
	// 🎉 All operations visualized in real-time!
}

func ExampleLongRunningControllerVisualization() {
	fmt.Println("=== Long-Running Controller Visualization ===")

	// Simulate a long-running controller with periodic reconciliation
	periodicController := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("⏰ Starting reconciliation cycle")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				// Simulate checking if resources need reconciliation
				return ctx.Value("needsReconciliation").(bool)
			},
			// Needs reconciliation
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("🔄 Resources out of sync - reconciling")
				}),
				state.Parallel(
					state.Action(func(ctx context.Context) {
						fmt.Println("🏗️  Reconciling deployments")
						time.Sleep(100 * time.Millisecond) // Simulate work
					}),
					state.Action(func(ctx context.Context) {
						fmt.Println("🌐 Reconciling services")
						time.Sleep(80 * time.Millisecond) // Simulate work
					}),
					state.Action(func(ctx context.Context) {
						fmt.Println("📝 Reconciling configmaps")
						time.Sleep(60 * time.Millisecond) // Simulate work
					}),
				),
			),
			// No reconciliation needed
			state.Action(func(ctx context.Context) {
				fmt.Println("✅ Resources in sync - no action needed")
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("😴 Waiting for next reconciliation cycle")
		}),
	)

	// Add real-time inspection
	inspector, inspectedController := WithRealTimeInspection("periodic-controller", periodicController)

	// Start inspector server
	go inspector.StartServer(":8082")

	fmt.Println("📡 Long-running controller inspector started!")
	fmt.Println("🔗 Live visualization: http://localhost:8082")

	// Simulate multiple reconciliation cycles
	cycles := []bool{true, false, true, false, true} // needs reconciliation?

	for i, needsRecon := range cycles {
		fmt.Printf("\n--- Reconciliation Cycle #%d ---\n", i+1)
		ctx := context.WithValue(context.Background(), "needsReconciliation", needsRecon)
		state.RunNewStage(ctx, inspectedController)

		// Pause between cycles (in real controller, this would be the reconcile interval)
		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("\n🎯 Long-running controller visualization complete!")
	fmt.Println("💡 The inspector showed real-time state transitions across multiple cycles")

	// Output:
	// === Long-Running Controller Visualization ===
	// 📡 Long-running controller inspector started!
	// 🔗 Live visualization: http://localhost:8082
	//
	// --- Reconciliation Cycle #1 ---
	// ⏰ Starting reconciliation cycle
	// 🔄 Resources out of sync - reconciling
	// 🏗️  Reconciling deployments
	// 🌐 Reconciling services
	// 📝 Reconciling configmaps
	// 😴 Waiting for next reconciliation cycle
	//
	// --- Reconciliation Cycle #2 ---
	// ⏰ Starting reconciliation cycle
	// ✅ Resources in sync - no action needed
	// 😴 Waiting for next reconciliation cycle
	//
	// --- Reconciliation Cycle #3 ---
	// ⏰ Starting reconciliation cycle
	// 🔄 Resources out of sync - reconciling
	// 🏗️  Reconciling deployments
	// 🌐 Reconciling services
	// 📝 Reconciling configmaps
	// 😴 Waiting for next reconciliation cycle
	//
	// --- Reconciliation Cycle #4 ---
	// ⏰ Starting reconciliation cycle
	// ✅ Resources in sync - no action needed
	// 😴 Waiting for next reconciliation cycle
	//
	// --- Reconciliation Cycle #5 ---
	// ⏰ Starting reconciliation cycle
	// 🔄 Resources out of sync - reconciling
	// 🏗️  Reconciling deployments
	// 🌐 Reconciling services
	// 📝 Reconciling configmaps
	// 😴 Waiting for next reconciliation cycle
	//
	// 🎯 Long-running controller visualization complete!
	// 💡 The inspector showed real-time state transitions across multiple cycles
}

func ExampleErrorHandlingVisualization() {
	fmt.Println("=== Error Handling Visualization ===")

	// Controller with error handling paths
	errorProneController := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("🎬 Starting operation")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				// Simulate random failures
				return ctx.Value("simulateError").(bool)
			},
			// Error path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("❌ Error detected!")
				}),
				state.Switch(
					func(ctx context.Context) string {
						return ctx.Value("errorType").(string)
					},
					map[string]state.NewStage{
						"retryable": state.Action(func(ctx context.Context) {
							fmt.Println("🔄 Retrying operation")
						}),
						"permanent": state.Action(func(ctx context.Context) {
							fmt.Println("💥 Permanent failure - giving up")
						}),
						"timeout": state.Action(func(ctx context.Context) {
							fmt.Println("⏱️  Timeout - will retry later")
						}),
					},
					state.Action(func(ctx context.Context) {
						fmt.Println("❓ Unknown error type")
					}),
				),
			),
			// Success path
			state.Action(func(ctx context.Context) {
				fmt.Println("✅ Operation completed successfully")
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("🏁 Operation finished")
		}),
	)

	// Add real-time inspection
	inspector, inspectedController := WithRealTimeInspection("error-handling-controller", errorProneController)

	go inspector.StartServer(":8083")

	fmt.Println("🚨 Error Handling Controller Inspector Started!")
	fmt.Println("🔗 Watch error flows at: http://localhost:8083")

	// Test different error scenarios
	scenarios := []struct {
		name      string
		hasError  bool
		errorType string
	}{
		{"Success", false, ""},
		{"Retryable Error", true, "retryable"},
		{"Permanent Error", true, "permanent"},
		{"Timeout Error", true, "timeout"},
		{"Unknown Error", true, "mystery"},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- Testing: %s ---\n", scenario.name)
		ctx := context.WithValue(context.Background(), "simulateError", scenario.hasError)
		ctx = context.WithValue(ctx, "errorType", scenario.errorType)
		state.RunNewStage(ctx, inspectedController)

		time.Sleep(250 * time.Millisecond)
	}

	fmt.Println("\n🎭 Error handling visualization complete!")
	fmt.Println("📊 The inspector visualized all error paths and recovery strategies")

	// Output:
	// === Error Handling Visualization ===
	// 🚨 Error Handling Controller Inspector Started!
	// 🔗 Watch error flows at: http://localhost:8083
	//
	// --- Testing: Success ---
	// 🎬 Starting operation
	// ✅ Operation completed successfully
	// 🏁 Operation finished
	//
	// --- Testing: Retryable Error ---
	// 🎬 Starting operation
	// ❌ Error detected!
	// 🔄 Retrying operation
	// 🏁 Operation finished
	//
	// --- Testing: Permanent Error ---
	// 🎬 Starting operation
	// ❌ Error detected!
	// 💥 Permanent failure - giving up
	// 🏁 Operation finished
	//
	// --- Testing: Timeout Error ---
	// 🎬 Starting operation
	// ❌ Error detected!
	// ⏱️  Timeout - will retry later
	// 🏁 Operation finished
	//
	// --- Testing: Unknown Error ---
	// 🎬 Starting operation
	// ❌ Error detected!
	// ❓ Unknown error type
	// 🏁 Operation finished
	//
	// 🎭 Error handling visualization complete!
	// 📊 The inspector visualized all error paths and recovery strategies
}
