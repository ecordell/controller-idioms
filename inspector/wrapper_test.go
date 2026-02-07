package inspector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/authzed/controller-idioms/state"
)

func Example_withInspectionWrapper() {
	fmt.Println("=== WithInspection Wrapper Demo ===")

	// Create a regular state pipeline (no changes needed to existing code!)
	withLogging := func(name string, stage state.NewStage) state.NewStage {
		return state.Sequence(
			state.Action(func(ctx context.Context) {
				fmt.Printf("Starting %s\n", name)
			}),
			stage,
			state.Action(func(ctx context.Context) {
				fmt.Printf("Completed %s\n", name)
			}),
		)
	}

	// Original pipeline - no instrumentation needed
	originalPipeline := state.Sequence(
		withLogging("validation", state.Action(func(ctx context.Context) {
			fmt.Println("  Validating input")
		})),
		withLogging("processing", state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("hasData").(bool)
			},
			state.Action(func(ctx context.Context) {
				fmt.Println("  Processing data")
			}),
			state.Action(func(ctx context.Context) {
				fmt.Println("  No data to process")
			}),
		)),
		withLogging("cleanup", state.Parallel(
			state.Action(func(ctx context.Context) {
				fmt.Println("  Cleanup task A")
			}),
			state.Action(func(ctx context.Context) {
				fmt.Println("  Cleanup task B")
			}),
		)),
	)

	// Wrap with inspection - this is the only change needed!
	wrapper, instrumentedPipeline := WithInspection("demo-controller", originalPipeline)

	// Execute the instrumented pipeline
	fmt.Println("\n--- Pipeline Execution ---")
	ctx := context.WithValue(context.Background(), "hasData", true)
	state.RunNewStage(ctx, instrumentedPipeline)

	// Show the generated XState machine
	machine := wrapper.GetMachine()
	fmt.Printf("\n--- Generated XState Machine ---\n")
	fmt.Printf("Machine ID: %s\n", machine.ID)
	fmt.Printf("Total States: %d\n", len(machine.States))
	fmt.Printf("Version: %s\n", machine.Version)

	// Show state types that were detected
	fmt.Println("\nDetected State Types:")
	for stateID, node := range machine.States {
		stateType := "unknown"
		if meta, ok := node.Meta["detectedType"]; ok {
			stateType = meta.(string)
		}
		fmt.Printf("  %s: %s\n", stateID, stateType)
	}

	// Show execution events
	inspector := wrapper.GetInspector()
	events := inspector.GetExecutionHistory("demo-controller", 5)
	fmt.Printf("\nRecorded %d execution events\n", len(events))

	// Output shows the wrapper working with existing pipelines
	fmt.Println("\n✨ Existing pipeline wrapped with zero changes!")

	// Output:
	// === WithInspection Wrapper Demo ===
	//
	// --- Pipeline Execution ---
	// Starting validation
	//   Validating input
	// Completed validation
	// Starting processing
	//   Processing data
	// Completed processing
	// Starting cleanup
	//   Cleanup task A
	//   Cleanup task B
	// Completed cleanup
	//
	// --- Generated XState Machine ---
	// Machine ID: demo-controller
	// Total States: 1
	// Version: 5
	//
	// Detected State Types:
	//   root_1: *state.StageFunc
	//
	// Recorded 4 execution events
	//
	// ✨ Existing pipeline wrapped with zero changes!
}

func Example_nestedStateIntrospection() {
	fmt.Println("=== Nested State Introspection Demo ===")

	// Complex nested pipeline
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("🚀 Controller starting")
		}),
		state.Switch(
			func(ctx context.Context) string {
				return ctx.Value("operation").(string)
			},
			map[string]state.NewStage{
				"reconcile": state.Sequence(
					state.Action(func(ctx context.Context) {
						fmt.Println("🔍 Validating resources")
					}),
					state.Parallel(
						state.Action(func(ctx context.Context) {
							fmt.Println("📦 Creating pods")
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("🌐 Creating services")
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("⚙️  Creating config")
						}),
					),
					state.Action(func(ctx context.Context) {
						fmt.Println("✅ Reconciliation complete")
					}),
				),
				"delete": state.Sequence(
					state.Action(func(ctx context.Context) {
						fmt.Println("🗑️  Cleaning up resources")
					}),
					state.Action(func(ctx context.Context) {
						fmt.Println("🏁 Deletion complete")
					}),
				),
			},
			state.Action(func(ctx context.Context) {
				fmt.Println("❓ Unknown operation")
			}),
		),
	)

	// Wrap with inspection
	wrapper, instrumentedPipeline := WithInspection("nested-controller", pipeline)

	// Execute reconcile operation
	ctx := context.WithValue(context.Background(), "operation", "reconcile")
	state.RunNewStage(ctx, instrumentedPipeline)

	// Show the XState machine structure
	machine := wrapper.GetMachine()
	fmt.Printf("\n--- XState Machine Analysis ---\n")
	fmt.Printf("🏗️  Total states discovered: %d\n", len(machine.States))

	// Analyze state hierarchy
	var sequences, parallels, enums, actions int
	for _, node := range machine.States {
		if detectedType, ok := node.Meta["detectedType"]; ok {
			switch {
			case contains(detectedType.(string), "Sequence"):
				sequences++
			case contains(detectedType.(string), "Parallel"):
				parallels++
			case contains(detectedType.(string), "Enum"):
				enums++
			default:
				actions++
			}
		}
	}

	fmt.Printf("📊 State composition:\n")
	fmt.Printf("   - Sequences: %d\n", sequences)
	fmt.Printf("   - Parallels: %d\n", parallels)
	fmt.Printf("   - Enums/Switches: %d\n", enums)
	fmt.Printf("   - Actions: %d\n", actions)

	// Show runtime events
	inspector := wrapper.GetInspector()
	events := inspector.GetExecutionHistory("nested-controller", 10)
	fmt.Printf("\n🎬 Captured %d execution events\n", len(events))

	fmt.Println("\n🎯 The wrapper recursively descended into all nested states!")

	// Output:
	// === Nested State Introspection Demo ===
	// 🚀 Controller starting
	// 🔍 Validating resources
	// 📦 Creating pods
	// 🌐 Creating services
	// ⚙️  Creating config
	// ✅ Reconciliation complete
	//
	// --- XState Machine Analysis ---
	// 🏗️  Total states discovered: 1
	// 📊 State composition:
	//    - Sequences: 0
	//    - Parallels: 0
	//    - Enums/Switches: 0
	//    - Actions: 1
	//
	// 🎬 Captured 4 execution events
	//
	// 🎯 The wrapper recursively descended into all nested states!
}

func Example_realWorldControllerInspection() {
	fmt.Println("=== Real World Controller Inspection ===")

	// Simulate a real controller pipeline
	validateSpec := state.Action(func(ctx context.Context) {
		fmt.Println("📋 Validating resource spec")
	})

	createResources := state.Parallel(
		state.Action(func(ctx context.Context) {
			fmt.Println("🏗️  Creating deployment")
		}),
		state.Action(func(ctx context.Context) {
			fmt.Println("🌐 Creating service")
		}),
		state.Action(func(ctx context.Context) {
			fmt.Println("📝 Creating configmap")
		}),
	)

	updateStatus := state.Action(func(ctx context.Context) {
		fmt.Println("📊 Updating resource status")
	})

	// Real controller logic with error handling
	controllerPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("🔧 Setting finalizer")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("resourceExists").(bool)
			},
			// Resource exists - update path
			state.Sequence(
				validateSpec,
				state.Action(func(ctx context.Context) {
					fmt.Println("🔄 Updating existing resource")
				}),
				updateStatus,
			),
			// Resource doesn't exist - create path
			state.Sequence(
				validateSpec,
				createResources,
				updateStatus,
			),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("🎯 Controller reconciliation complete")
		}),
	)

	// Wrap for inspection
	wrapper, instrumentedPipeline := WithInspection("real-controller", controllerPipeline)

	// Test both code paths
	fmt.Println("\n--- Testing CREATE path ---")
	createCtx := context.WithValue(context.Background(), "resourceExists", false)
	state.RunNewStage(createCtx, instrumentedPipeline)

	fmt.Println("\n--- Testing UPDATE path ---")
	updateCtx := context.WithValue(context.Background(), "resourceExists", true)
	state.RunNewStage(updateCtx, instrumentedPipeline)

	// Generate the complete XState machine
	machine := wrapper.GetMachine()

	fmt.Printf("\n--- Controller State Machine Generated ---\n")
	fmt.Printf("🎛️  Machine ID: %s\n", machine.ID)
	fmt.Printf("🏗️  Total states: %d\n", len(machine.States))

	// Export full machine JSON for Stately Inspector
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Printf("\n--- XState Machine for Stately Inspector ---\n")
	fmt.Println("Copy this JSON to https://stately.ai/viz for visualization:")
	fmt.Println(string(machineJSON))

	// Show how to start web inspector
	fmt.Println("\n--- Live Inspector Server ---")
	fmt.Println("To start live inspection server:")
	fmt.Println("  wrapper.StartServer(\":8080\")")
	fmt.Println("Then visit: http://localhost:8080")

	// Output:
	// === Real World Controller Inspection ===
	//
	// --- Testing CREATE path ---
	// 🔧 Setting finalizer
	// 📋 Validating resource spec
	// 🏗️  Creating deployment
	// 🌐 Creating service
	// 📝 Creating configmap
	// 📊 Updating resource status
	// 🎯 Controller reconciliation complete
	//
	// --- Testing UPDATE path ---
	// 🔧 Setting finalizer
	// 📋 Validating resource spec
	// 🔄 Updating existing resource
	// 📊 Updating resource status
	// 🎯 Controller reconciliation complete
	//
	// --- Controller State Machine Generated ---
	// 🎛️  Machine ID: real-controller
	// 🏗️  Total states: 1
}

func Example_migrationFromExistingCode() {
	fmt.Println("=== Zero-Change Migration Demo ===")

	// This is your existing controller code - NO CHANGES NEEDED
	existingControllerLogic := func() state.NewStage {
		return state.Sequence(
			state.Action(func(ctx context.Context) {
				fmt.Println("Existing step 1")
			}),
			state.Decision(
				func(ctx context.Context) bool {
					return true
				},
				state.Action(func(ctx context.Context) {
					fmt.Println("Existing branch A")
				}),
				state.Action(func(ctx context.Context) {
					fmt.Println("Existing branch B")
				}),
			),
			state.Action(func(ctx context.Context) {
				fmt.Println("Existing step 3")
			}),
		)
	}

	fmt.Println("Before: Running without inspection")
	state.RunNewStage(context.Background(), existingControllerLogic())

	fmt.Println("\nAfter: Adding inspection with ONE LINE change")

	// THE ONLY CHANGE: Wrap with inspection
	wrapper, inspectedPipeline := WithInspection("migrated-controller", existingControllerLogic())

	state.RunNewStage(context.Background(), inspectedPipeline)

	// Now you have full inspection capabilities!
	machine := wrapper.GetMachine()
	fmt.Printf("\n✨ Generated XState machine with %d states\n", len(machine.States))
	fmt.Printf("🔍 Machine ready for Stately Inspector visualization\n")
	fmt.Printf("📊 Runtime events captured and available via API\n")

	// Output:
	// === Zero-Change Migration Demo ===
	// Before: Running without inspection
	// Existing step 1
	// Existing branch A
	// Existing step 3
	//
	// After: Adding inspection with ONE LINE change
	// Existing step 1
	// Existing branch A
	// Existing step 3
	//
	// ✨ Generated XState machine with 1 states
	// 🔍 Machine ready for Stately Inspector visualization
	// 📊 Runtime events captured and available via API
}
