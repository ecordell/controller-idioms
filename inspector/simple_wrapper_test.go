package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/authzed/controller-idioms/state"
)

func TestWithInspectionBasicFunctionality(t *testing.T) {
	// Create a simple pipeline
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			// Simple action
		}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {
				// True branch
			}),
			state.Action(func(ctx context.Context) {
				// False branch
			}),
		),
	)

	// Wrap with inspection
	wrapper, instrumentedPipeline := WithInspection("test-controller", pipeline)

	if wrapper == nil {
		t.Fatal("wrapper should not be nil")
	}

	if instrumentedPipeline == nil {
		t.Fatal("instrumented pipeline should not be nil")
	}

	// Execute the pipeline
	state.RunNewStage(context.Background(), instrumentedPipeline)

	// Verify machine was generated
	machine := wrapper.GetMachine()
	if machine == nil {
		t.Fatal("machine should not be nil")
	}

	if machine.ID != "test-controller" {
		t.Errorf("expected machine ID 'test-controller', got %s", machine.ID)
	}

	if machine.Version != "5" {
		t.Errorf("expected XState version '5', got %s", machine.Version)
	}

	if len(machine.States) == 0 {
		t.Error("expected at least one state in machine")
	}

	// Verify events were recorded
	inspector := wrapper.GetInspector()
	events := inspector.GetExecutionHistory("test-controller", 10)
	if len(events) == 0 {
		t.Error("expected execution events to be recorded")
	}
}

func ExampleWithInspection() {
	fmt.Println("=== WithInspection Demo ===")

	// Original pipeline - no changes needed!
	originalPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("Step 1: Initialize")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("success").(bool)
			},
			state.Action(func(ctx context.Context) {
				fmt.Println("Step 2: Success path")
			}),
			state.Action(func(ctx context.Context) {
				fmt.Println("Step 2: Error path")
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("Step 3: Cleanup")
		}),
	)

	// Wrap for inspection
	wrapper, inspectedPipeline := WithInspection("demo", originalPipeline)

	// Execute
	ctx := context.WithValue(context.Background(), "success", true)
	state.RunNewStage(ctx, inspectedPipeline)

	// Show results
	machine := wrapper.GetMachine()
	inspector := wrapper.GetInspector()
	events := inspector.GetExecutionHistory("demo", 5)

	fmt.Printf("\nGenerated XState machine '%s' with %d states\n", machine.ID, len(machine.States))
	fmt.Printf("Recorded %d execution events\n", len(events))
	fmt.Println("\nXState JSON (for Stately Inspector):")

	// Show the actual XState output - this is the key value!
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Println(string(machineJSON))

	// Output:
	// === WithInspection Demo ===
	// Step 1: Initialize
	// Step 2: Success path
	// Step 3: Cleanup
	//
	// Generated XState machine 'demo' with 1 states
	// Recorded 2 execution events
	//
	// XState JSON (for Stately Inspector):
	// {
	//   "id": "demo",
	//   "initial": "root",
	//   "states": {
	//     "root": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "stageID": "root"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "detectedType": "action",
	//         "stageID": "root",
	//         "type": "action"
	//       }
	//     }
	//   },
	//   "version": "5"
	// }
}

func Example_inspectionWithoutChanges() {
	// This shows how you can add inspection to existing controllers
	// with literally ONE LINE of change

	fmt.Println("=== Zero-Change Controller Inspection ===")

	// Your existing controller function - NO CHANGES
	buildControllerPipeline := func() state.NewStage {
		return state.Sequence(
			state.Action(func(ctx context.Context) {
				fmt.Println("🔧 Setting finalizer")
			}),
			state.Switch(
				func(ctx context.Context) string {
					return ctx.Value("operation").(string)
				},
				map[string]state.NewStage{
					"create": state.Action(func(ctx context.Context) {
						fmt.Println("➕ Creating resource")
					}),
					"update": state.Action(func(ctx context.Context) {
						fmt.Println("✏️  Updating resource")
					}),
					"delete": state.Action(func(ctx context.Context) {
						fmt.Println("🗑️  Deleting resource")
					}),
				},
				state.Action(func(ctx context.Context) {
					fmt.Println("❓ Unknown operation")
				}),
			),
			state.Action(func(ctx context.Context) {
				fmt.Println("✅ Controller finished")
			}),
		)
	}

	// BEFORE: Regular execution
	fmt.Println("Before (no inspection):")
	ctx := context.WithValue(context.Background(), "operation", "create")
	state.RunNewStage(ctx, buildControllerPipeline())

	// AFTER: Add inspection with ONE line change
	fmt.Println("\nAfter (with inspection):")
	wrapper, inspectedPipeline := WithInspection("my-controller", buildControllerPipeline())
	state.RunNewStage(ctx, inspectedPipeline)

	// Now you have full state machine introspection!
	machine := wrapper.GetMachine()
	fmt.Printf("\n🎉 XState machine generated: %s\n", machine.ID)
	fmt.Printf("📊 States discovered: %d\n", len(machine.States))
	fmt.Printf("🔍 Ready for Stately Inspector visualization!\n")

	// Output:
	// === Zero-Change Controller Inspection ===
	// Before (no inspection):
	// 🔧 Setting finalizer
	// ➕ Creating resource
	// ✅ Controller finished
	//
	// After (with inspection):
	// 🔧 Setting finalizer
	// ➕ Creating resource
	// ✅ Controller finished
	//
	// 🎉 XState machine generated: my-controller
	// 📊 States discovered: 1
	// 🔍 Ready for Stately Inspector visualization!
}
