package inspector

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/authzed/controller-idioms/state"
)

func Example() {
	// Create a new inspector
	inspector := NewInspector()

	// Define a complex controller state machine
	controllerPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("Setting finalizer")
		}),
		state.Action(func(ctx context.Context) {
			fmt.Println("Checking pause state")
		}),
		state.Switch(
			func(ctx context.Context) string {
				// Simulate getting resource type from context
				return ctx.Value("resourceType").(string)
			},
			map[string]state.NewStage{
				"deployment": state.Sequence(
					state.Action(func(ctx context.Context) {
						fmt.Println("Validating deployment spec")
					}),
					state.Parallel(
						state.Action(func(ctx context.Context) {
							fmt.Println("Creating pods")
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("Creating services")
						}),
					),
				),
				"service": state.Action(func(ctx context.Context) {
					fmt.Println("Processing service")
				}),
				"configmap": state.Action(func(ctx context.Context) {
					fmt.Println("Processing configmap")
				}),
			},
			state.Action(func(ctx context.Context) {
				fmt.Println("Unknown resource type")
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("Finalizing reconciliation")
		}),
	)

	// Analyze the state machine structure
	machine, err := inspector.AnalyzeStage("controller-main", controllerPipeline)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated XState machine: %s\n", machine.ID)

	// Instrument the pipeline for live inspection
	instrumentedPipeline := inspector.InstrumentStage("controller-main", "root", controllerPipeline)

	// Execute the instrumented pipeline
	ctx := context.WithValue(context.Background(), "resourceType", "deployment")
	state.RunNewStage(ctx, instrumentedPipeline)

	// Get execution history
	history := inspector.GetExecutionHistory("controller-main", 10)
	fmt.Printf("Recorded %d+ execution events\n", len(history))

	// Start the web inspector (in a real scenario, this would be in a goroutine)
	fmt.Println("Starting inspector server at http://localhost:8080")
	fmt.Println("Visit http://localhost:8080 to inspect the state machine")

	// In a real application, you'd start this in a goroutine:
	// go inspector.StartServer(":8080")

	// Output:
	// Generated XState machine: controller-main
	// Setting finalizer
	// Checking pause state
	// Validating deployment spec
	// Finalizing reconciliation
	// Recorded 2+ execution events
	// Starting inspector server at http://localhost:8080
	// Visit http://localhost:8080 to inspect the state machine
}

func Example_complexStateMachine() {
	inspector := NewInspector()

	// Define a more complex state machine with error handling
	complexPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("Initializing")
		}),
		state.CallAndContinueIf(
			state.Action(func(ctx context.Context) {
				fmt.Println("Running validation")
				// Simulate validation that might fail
			}),
			func(ctx context.Context) bool {
				// Check if validation succeeded
				return ctx.Err() == nil
			},
			// Success path
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("Validation passed")
				}),
				state.Enum(
					func(ctx context.Context) string {
						return ctx.Value("operation").(string)
					},
					map[string]state.NewStage{
						"create": state.Action(func(ctx context.Context) {
							fmt.Println("Creating resource")
						}),
						"update": state.Action(func(ctx context.Context) {
							fmt.Println("Updating resource")
						}),
						"delete": state.Action(func(ctx context.Context) {
							fmt.Println("Deleting resource")
						}),
					},
					state.Action(func(ctx context.Context) {
						fmt.Println("Unknown operation")
					}),
				),
			),
			// Failure path
			state.Action(func(ctx context.Context) {
				fmt.Println("Validation failed - running recovery")
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("Cleanup completed")
		}),
	)

	// Analyze and instrument
	machine, err := inspector.AnalyzeStage("complex-controller", complexPipeline)
	if err != nil {
		log.Fatal(err)
	}

	instrumentedPipeline := inspector.InstrumentStage("complex-controller", "root", complexPipeline)

	// Execute with different contexts
	fmt.Println("=== Executing CREATE operation ===")
	ctx := context.WithValue(context.Background(), "operation", "create")
	state.RunNewStage(ctx, instrumentedPipeline)

	fmt.Println("\n=== Executing UPDATE operation ===")
	ctx = context.WithValue(context.Background(), "operation", "update")
	state.RunNewStage(ctx, instrumentedPipeline)

	// Show machine structure
	fmt.Printf("\nMachine ID: %s\n", machine.ID)
	fmt.Printf("Initial state: %s\n", machine.Initial)
	fmt.Printf("Number of states: %d\n", len(machine.States))

	// Output:
	// === Executing CREATE operation ===
	// Initializing
	// Running validation
	// Validation passed
	// Creating resource
	// Cleanup completed
	//
	// === Executing UPDATE operation ===
	// Initializing
	// Running validation
	// Validation passed
	// Updating resource
	// Cleanup completed
	//
	// Machine ID: complex-controller
	// Initial state: root
	// Number of states: 1+
}

func Example_realTimeInspection() {
	inspector := NewInspector()

	// Create a pipeline with delays to simulate real work
	slowPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("Starting long-running process")
			time.Sleep(100 * time.Millisecond)
		}),
		state.Parallel(
			state.Action(func(ctx context.Context) {
				fmt.Println("Background task 1")
				time.Sleep(200 * time.Millisecond)
			}),
			state.Action(func(ctx context.Context) {
				fmt.Println("Background task 2")
				time.Sleep(150 * time.Millisecond)
			}),
		),
		state.Action(func(ctx context.Context) {
			fmt.Println("Process completed")
		}),
	)

	// Analyze the pipeline
	inspector.AnalyzeStage("slow-controller", slowPipeline)

	// Instrument for real-time tracking
	instrumentedPipeline := inspector.InstrumentStage("slow-controller", "root", slowPipeline)

	// Execute in background
	go func() {
		state.RunNewStage(context.Background(), instrumentedPipeline)
	}()

	// Simulate periodic inspection
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)

		history := inspector.GetExecutionHistory("slow-controller", 3)
		fmt.Printf("Recent events: %d+\n", len(history))
		fmt.Println("---")
	}

	// Output:
	// Starting long-running process
	// Recent events: 2+
	// ---
	// Process completed
	// Recent events: 2+
	// ---
	// Recent events: 2+
	// ---
	// Recent events: 2+
	// ---
	// Recent events: 2+
	// ---
}

func Example_webInspector() {
	inspector := NewInspector()

	// Create a sample controller pipeline
	controllerPipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			fmt.Println("Controller started")
		}),
		state.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("hasResource").(bool)
			},
			state.Action(func(ctx context.Context) {
				fmt.Println("Resource exists - updating")
			}),
			state.Action(func(ctx context.Context) {
				fmt.Println("Resource missing - creating")
			}),
		),
	)

	// Register the pipeline
	inspector.AnalyzeStage("web-demo", controllerPipeline)
	instrumentedPipeline := inspector.InstrumentStage("web-demo", "root", controllerPipeline)

	// Execute with different scenarios
	ctx1 := context.WithValue(context.Background(), "hasResource", true)
	state.RunNewStage(ctx1, instrumentedPipeline)

	ctx2 := context.WithValue(context.Background(), "hasResource", false)
	state.RunNewStage(ctx2, instrumentedPipeline)

	fmt.Println("Web inspector available at http://localhost:8080")
	fmt.Println("The inspector provides:")
	fmt.Println("- GET /api/machines - XState machine definitions")
	fmt.Println("- GET /api/state - Current states of all machines")
	fmt.Println("- GET /api/history?machine=web-demo&limit=10 - Execution history")
	fmt.Println("- GET / - Web UI for visual inspection")

	// In production, you would start the server:
	// log.Fatal(inspector.StartServer(":8080"))

	// Output:
	// Controller started
	// Resource exists - updating
	// Controller started
	// Resource missing - creating
	// Web inspector available at http://localhost:8080
	// The inspector provides:
	// - GET /api/machines - XState machine definitions
	// - GET /api/state - Current states of all machines
	// - GET /api/history?machine=web-demo&limit=10 - Execution history
	// - GET / - Web UI for visual inspection
}
