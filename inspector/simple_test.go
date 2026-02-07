package inspector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/authzed/controller-idioms/state"
)

func Example_simpleXStateMachine() {
	// Create an instrumented builder
	builder := NewInstrumentedBuilder("simple-controller")

	// Build a simple, clear state machine
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Step 1: Initialize")
		}),
		builder.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("success").(bool)
			},
			builder.Action(func(ctx context.Context) {
				fmt.Println("Step 2a: Success path")
			}),
			builder.Action(func(ctx context.Context) {
				fmt.Println("Step 2b: Failure path")
			}),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Step 3: Finalize")
		}),
	)

	// Generate the XState machine
	machine := builder.BuildMachine()

	// Execute the pipeline
	ctx := context.WithValue(context.Background(), "success", true)
	state.RunNewStage(ctx, pipeline)

	fmt.Printf("\n=== Generated XState Machine ===\n")
	fmt.Printf("ID: %s\n", machine.ID)
	fmt.Printf("Initial State: %s\n", machine.Initial)
	fmt.Printf("Total States: %d\n", len(machine.States))

	// Print the complete machine as JSON
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Printf("\nComplete XState JSON:\n%s\n", string(machineJSON))

	// Output:
	// Step 1: Initialize
	// Step 2a: Success path
	// Step 3: Finalize
	//
	// === Generated XState Machine ===
	// ID: simple-controller
	// Initial State: sequence_1
	// Total States: 6
	//
	// Complete XState JSON:
	// {
	//   "id": "simple-controller",
	//   "initial": "sequence_1",
	//   "states": {
	//     "action_10": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "actionID": "action_10"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "type": "action"
	//       }
	//     },
	//     "action_2": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "actionID": "action_2"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "type": "action"
	//       }
	//     },
	//     "action_7": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "actionID": "action_7"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "type": "action"
	//       }
	//     },
	//     "action_8": {
	//       "type": "atomic",
	//       "entry": [
	//         {
	//           "type": "executeAction",
	//           "meta": {
	//             "actionID": "action_8"
	//           }
	//         }
	//       ],
	//       "meta": {
	//         "type": "action"
	//       }
	//     },
	//     "decision_4": {
	//       "type": "atomic",
	//       "on": {
	//         "EVALUATE": {
	//           "guard": {
	//             "type": "evaluateCondition",
	//             "meta": {
	//               "falseTarget": "decision_false_6",
	//               "trueTarget": "decision_true_5"
	//             }
	//           }
	//         }
	//       },
	//       "meta": {
	//         "type": "decision"
	//       }
	//     },
	//     "sequence_1": {
	//       "type": "compound",
	//       "initial": "seq_step_0_3",
	//       "states": {
	//         "seq_step_0_3": {
	//           "type": "atomic",
	//           "always": [
	//             {
	//               "target": "seq_step_1_9"
	//             }
	//           ],
	//           "meta": {
	//             "sequenceIndex": 0
	//           }
	//         },
	//         "seq_step_1_9": {
	//           "type": "atomic",
	//           "always": [
	//             {
	//               "target": "seq_step_2_11"
	//             }
	//           ],
	//           "meta": {
	//             "sequenceIndex": 1
	//           }
	//         },
	//         "seq_step_2_11": {
	//           "type": "atomic",
	//           "meta": {
	//             "sequenceIndex": 2
	//           }
	//         }
	//       },
	//       "meta": {
	//         "stageCount": 3,
	//         "type": "sequence"
	//       }
	//     }
	//   },
	//   "version": "5"
	// }
}

func Example_parallelXStateMachine() {
	builder := NewInstrumentedBuilder("parallel-demo")

	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Before parallel")
		}),
		builder.Parallel(
			builder.Action(func(ctx context.Context) {
				fmt.Println("Parallel task A")
			}),
			builder.Action(func(ctx context.Context) {
				fmt.Println("Parallel task B")
			}),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("After parallel")
		}),
	)

	machine := builder.BuildMachine()
	state.RunNewStage(context.Background(), pipeline)

	fmt.Printf("\n=== Parallel State Machine ===\n")

	// Find and show the parallel state structure
	for stateID, stateNode := range machine.States {
		if stateNode.Type == "parallel" {
			fmt.Printf("Found parallel state '%s' with %d branches:\n", stateID, len(stateNode.States))
			for branchID, branch := range stateNode.States {
				fmt.Printf("  Branch '%s': %s\n", branchID, branch.Type)
			}
		}
	}

	// Output:
	// Before parallel
	// After parallel
	//
	// === Parallel State Machine ===
	// Found parallel state 'parallel_5' with 2 branches:
	//   Branch 'par_branch_0_6': atomic
	//   Branch 'par_branch_1_7': atomic
}
