package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/authzed/controller-idioms/state"
)

func ExampleInstrumentedBuilder() {
	// Create an instrumented builder
	builder := NewInstrumentedBuilder("demo-controller")

	// Build a complex state machine using the instrumented builder
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Setting finalizer")
		}),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Checking pause state")
		}),
		builder.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("resourceExists").(bool)
			},
			// Resource exists - update path
			builder.Sequence(
				builder.Action(func(ctx context.Context) {
					fmt.Println("Resource exists - validating")
				}),
				builder.Action(func(ctx context.Context) {
					fmt.Println("Updating resource")
				}),
			),
			// Resource doesn't exist - create path
			builder.Sequence(
				builder.Action(func(ctx context.Context) {
					fmt.Println("Resource missing - creating")
				}),
				builder.Action(func(ctx context.Context) {
					fmt.Println("Creating resources")
				}),
			),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Finalizing reconciliation")
		}),
	)

	// Build the XState machine
	machine := builder.BuildMachine()

	// Execute the pipeline
	ctx := context.WithValue(context.Background(), "resourceExists", false)
	state.RunNewStage(ctx, pipeline)

	fmt.Printf("Generated XState machine with %d+ states\n", len(machine.States))
	fmt.Printf("Machine ID: %s\n", machine.ID)

	// Print the actual XState machine definition
	fmt.Println("\nGenerated XState Machine JSON:")
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Println(string(machineJSON))

	// Output:
	// Setting finalizer
	// Checking pause state
	// Resource missing - creating
	// Creating resources
	// Finalizing reconciliation
	// Generated XState machine with 21+ states
	// Machine ID: demo-controller
	//
	// Generated XState Machine JSON:
	// {
	//   "id": "demo-controller",
	//   "initial": "action_4",
	//   "states": {
	//     "action_1": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "actionID": "action_1"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "type": "action"
	//       }
	//     }
	//   },
	//   "version": "5"
	// }
}

func Example_complexControllerMachine() {
	builder := NewInstrumentedBuilder("complex-controller")

	// Build a sophisticated controller state machine
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Initializing controller")
		}),
		builder.Switch(
			func(ctx context.Context) string {
				return ctx.Value("operation").(string)
			},
			map[string]state.NewStage{
				"reconcile": builder.Sequence(
					builder.Action(func(ctx context.Context) {
						fmt.Println("Starting reconciliation")
					}),
					builder.Decision(
						func(ctx context.Context) bool {
							return ctx.Value("resourceHealthy").(bool)
						},
						builder.Action(func(ctx context.Context) {
							fmt.Println("Resource is healthy - monitoring")
						}),
						builder.Sequence(
							builder.Action(func(ctx context.Context) {
								fmt.Println("Resource unhealthy - diagnosing")
							}),
							builder.Parallel(
								builder.Action(func(ctx context.Context) {
									fmt.Println("Checking dependencies")
								}),
								builder.Action(func(ctx context.Context) {
									fmt.Println("Analyzing logs")
								}),
								builder.Action(func(ctx context.Context) {
									fmt.Println("Running health checks")
								}),
							),
							builder.Action(func(ctx context.Context) {
								fmt.Println("Attempting repair")
							}),
						),
					),
				),
				"delete": builder.Sequence(
					builder.Action(func(ctx context.Context) {
						fmt.Println("Starting deletion")
					}),
					builder.Action(func(ctx context.Context) {
						fmt.Println("Cleaning up resources")
					}),
					builder.Action(func(ctx context.Context) {
						fmt.Println("Removing finalizers")
					}),
				),
				"validate": builder.Action(func(ctx context.Context) {
					fmt.Println("Validating configuration")
				}),
			},
			builder.Action(func(ctx context.Context) {
				fmt.Println("Unknown operation")
			}),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Operation completed")
		}),
	)

	// Generate the XState machine
	machine := builder.BuildMachine()

	// Test reconcile path with unhealthy resource
	fmt.Println("=== Testing reconcile with unhealthy resource ===")
	ctx1 := context.WithValue(context.Background(), "operation", "reconcile")
	ctx1 = context.WithValue(ctx1, "resourceHealthy", false)
	state.RunNewStage(ctx1, pipeline)

	fmt.Println("\n=== Testing delete operation ===")
	ctx2 := context.WithValue(context.Background(), "operation", "delete")
	state.RunNewStage(ctx2, pipeline)

	// Show machine structure
	fmt.Printf("\nXState Machine Structure:\n")
	fmt.Printf("- Machine ID: %s\n", machine.ID)
	fmt.Printf("- Total states: %d+\n", len(machine.States))

	// Output:
	// === Testing reconcile with unhealthy resource ===
	// Initializing controller
	// Starting reconciliation
	// Resource unhealthy - diagnosing
	// Attempting repair
	// Operation completed
	//
	// === Testing delete operation ===
	// Initializing controller
	// Starting deletion
	// Cleaning up resources
	// Removing finalizers
	// Operation completed
	//
	// XState Machine Structure:
	// - Machine ID: complex-controller
	// - Total states: 41+
}

func Example_machineVisualization() {
	builder := NewInstrumentedBuilder("visualization-demo")

	// Create a simple but illustrative state machine
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Start")
		}),
		builder.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("condition").(bool)
			},
			builder.Action(func(ctx context.Context) {
				fmt.Println("True path")
			}),
			builder.Parallel(
				builder.Action(func(ctx context.Context) {
					fmt.Println("False path A")
				}),
				builder.Action(func(ctx context.Context) {
					fmt.Println("False path B")
				}),
			),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("End")
		}),
	)

	// Build and serialize the machine
	machine := builder.BuildMachine()

	// Execute the pipeline
	ctx := context.WithValue(context.Background(), "condition", false)
	state.RunNewStage(ctx, pipeline)

	// Show the generated XState JSON structure
	fmt.Printf("XState JSON (truncated):\n")
	fmt.Printf("{\n")
	fmt.Printf("  \"id\": \"%s\",\n", machine.ID)
	fmt.Printf("  \"version\": \"%s\",\n", machine.Version)
	fmt.Printf("  \"states\": { /* %d+ states */ }\n", len(machine.States))
	fmt.Printf("}\n")

	// Show that transitions were discovered
	transitions := builder.GetBuilder().GetTransitions()
	fmt.Printf("\nDiscovered %d+ transitions\n", len(transitions))

	// Output:
	// Start
	// End
	// XState JSON (truncated):
	// {
	//   "id": "visualization-demo",
	//   "version": "5",
	//   "states": { /* 15+ states */ }
	// }
	//
	// Discovered 4+ transitions
}

func TestBuilderStateGeneration(t *testing.T) {
	builder := NewInstrumentedBuilder("test-machine")

	// Test that each stage type generates appropriate XState nodes
	testCases := []struct {
		name          string
		stageBuilder  func() state.NewStage
		expectedNodes int
	}{
		{
			"action",
			func() state.NewStage {
				return builder.Action(func(ctx context.Context) {})
			},
			1,
		},
		{
			"sequence",
			func() state.NewStage {
				return builder.Sequence(
					builder.Action(func(ctx context.Context) {}),
					builder.Action(func(ctx context.Context) {}),
				)
			},
			3, // sequence + 2 actions
		},
		{
			"parallel",
			func() state.NewStage {
				return builder.Parallel(
					builder.Action(func(ctx context.Context) {}),
					builder.Action(func(ctx context.Context) {}),
				)
			},
			3, // parallel + 2 actions
		},
		{
			"decision",
			func() state.NewStage {
				return builder.Decision(
					func(ctx context.Context) bool { return true },
					builder.Action(func(ctx context.Context) {}),
					builder.Action(func(ctx context.Context) {}),
				)
			},
			3, // decision + 2 actions
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset builder for each test
			builder = NewInstrumentedBuilder(fmt.Sprintf("test-%s", tc.name))

			// Build the stage
			stage := tc.stageBuilder()

			// Execute to trigger instrumentation
			state.RunNewStage(context.Background(), stage)

			// Check the generated machine
			machine := builder.BuildMachine()

			if len(machine.States) < tc.expectedNodes {
				t.Errorf("expected at least %d nodes, got %d", tc.expectedNodes, len(machine.States))
			}

			if machine.ID != fmt.Sprintf("test-%s", tc.name) {
				t.Errorf("expected machine ID 'test-%s', got '%s'", tc.name, machine.ID)
			}

			if machine.Version != "5" {
				t.Errorf("expected XState version '5', got '%s'", machine.Version)
			}
		})
	}
}

func TestBuilderTransitions(t *testing.T) {
	builder := NewInstrumentedBuilder("transition-test")

	// Create a linear sequence
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {}),
		builder.Action(func(ctx context.Context) {}),
		builder.Action(func(ctx context.Context) {}),
	)

	// Execute to discover transitions
	state.RunNewStage(context.Background(), pipeline)

	// Check transitions were recorded
	transitions := builder.GetBuilder().GetTransitions()

	if len(transitions) == 0 {
		t.Error("expected transitions to be recorded")
	}

	// Verify we have sequential transitions
	hasSequentialTransitions := false
	for from, tos := range transitions {
		if len(tos) > 0 && from != "" {
			hasSequentialTransitions = true
			break
		}
	}

	if !hasSequentialTransitions {
		t.Error("expected to find sequential transitions in the recorded data")
	}
}

func BenchmarkInstrumentedBuilder(b *testing.B) {
	builder := NewInstrumentedBuilder("benchmark")

	// Create a moderately complex pipeline
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {}),
		builder.Decision(
			func(ctx context.Context) bool { return true },
			builder.Parallel(
				builder.Action(func(ctx context.Context) {}),
				builder.Action(func(ctx context.Context) {}),
			),
			builder.Action(func(ctx context.Context) {}),
		),
		builder.Action(func(ctx context.Context) {}),
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		state.RunNewStage(context.Background(), pipeline)
	}
}
