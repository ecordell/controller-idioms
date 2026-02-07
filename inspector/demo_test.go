package inspector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/authzed/controller-idioms/state"
)

// This demo shows the XState machine generation in action
func Example_xStateDemo() {
	fmt.Println("=== XState Inspector Demo ===")

	// Create an instrumented builder
	builder := NewInstrumentedBuilder("demo-controller")

	// Build a representative controller state machine
	pipeline := builder.Sequence(
		builder.Action(func(ctx context.Context) {
			fmt.Println("🚀 Starting controller")
		}),
		builder.Decision(
			func(ctx context.Context) bool {
				return ctx.Value("resourceExists").(bool)
			},
			// Resource exists - update path
			builder.Action(func(ctx context.Context) {
				fmt.Println("✅ Resource exists - updating")
			}),
			// Resource doesn't exist - create path
			builder.Sequence(
				builder.Action(func(ctx context.Context) {
					fmt.Println("❌ Resource missing - creating")
				}),
				builder.Parallel(
					builder.Action(func(ctx context.Context) {
						fmt.Println("🔧 Setting up component A")
					}),
					builder.Action(func(ctx context.Context) {
						fmt.Println("🔧 Setting up component B")
					}),
				),
			),
		),
		builder.Action(func(ctx context.Context) {
			fmt.Println("🎯 Reconciliation complete")
		}),
	)

	// Generate the XState machine
	machine := builder.BuildMachine()

	// Execute the pipeline (resource doesn't exist path)
	fmt.Println("\n--- Executing Controller Logic ---")
	ctx := context.WithValue(context.Background(), "resourceExists", false)
	state.RunNewStage(ctx, pipeline)

	// Show the generated XState machine
	fmt.Printf("\n--- Generated XState Machine ---\n")
	fmt.Printf("Machine ID: %s\n", machine.ID)
	fmt.Printf("Version: %s\n", machine.Version)
	fmt.Printf("Initial State: %s\n", machine.Initial)
	fmt.Printf("Total States: %d\n", len(machine.States))

	// Show state structure
	fmt.Println("\nState Types:")
	for stateID, stateNode := range machine.States {
		stateType := "unknown"
		if meta, ok := stateNode.Meta["type"]; ok {
			stateType = meta.(string)
		}
		fmt.Printf("  %s: %s\n", stateID, stateType)
	}

	// Show the complete JSON (formatted nicely)
	fmt.Println("\n--- Complete XState JSON ---")
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Println(string(machineJSON))

	fmt.Println("\n✨ This XState machine can be imported into Stately Inspector for visualization!")

	// Output: Demo output showing XState generation
}

func Example_parallelDemo() {
	fmt.Println("=== Parallel State Machine Demo ===")

	builder := NewInstrumentedBuilder("parallel-demo")

	pipeline := builder.Parallel(
		builder.Action(func(ctx context.Context) {
			fmt.Println("Task A running")
		}),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Task B running")
		}),
		builder.Action(func(ctx context.Context) {
			fmt.Println("Task C running")
		}),
	)

	machine := builder.BuildMachine()
	state.RunNewStage(context.Background(), pipeline)

	fmt.Printf("\n--- Parallel State Structure ---\n")

	// Find the parallel state
	for stateID, stateNode := range machine.States {
		if stateNode.Type == "parallel" {
			fmt.Printf("Parallel state '%s' contains %d branches:\n", stateID, len(stateNode.States))
			for branchID := range stateNode.States {
				fmt.Printf("  - %s\n", branchID)
			}

			// Show JSON for the parallel state
			parallelJSON, _ := json.MarshalIndent(stateNode, "", "  ")
			fmt.Printf("\nParallel state JSON:\n%s\n", string(parallelJSON))
			break
		}
	}

	// Output: Parallel demo showing concurrent state structure
}

func Example_switchDemo() {
	fmt.Println("=== Switch/Enum State Machine Demo ===")

	builder := NewInstrumentedBuilder("switch-demo")

	pipeline := builder.Switch(
		func(ctx context.Context) string {
			return ctx.Value("operation").(string)
		},
		map[string]state.NewStage{
			"create": builder.Action(func(ctx context.Context) {
				fmt.Println("Creating resource")
			}),
			"update": builder.Action(func(ctx context.Context) {
				fmt.Println("Updating resource")
			}),
			"delete": builder.Action(func(ctx context.Context) {
				fmt.Println("Deleting resource")
			}),
		},
		builder.Action(func(ctx context.Context) {
			fmt.Println("Unknown operation")
		}),
	)

	machine := builder.BuildMachine()

	// Test create operation
	ctx := context.WithValue(context.Background(), "operation", "create")
	state.RunNewStage(ctx, pipeline)

	fmt.Printf("\n--- Switch/Enum State Structure ---\n")

	// Show the enum state
	for stateID, stateNode := range machine.States {
		if meta, ok := stateNode.Meta["type"]; ok && meta == "enum" {
			fmt.Printf("Enum state '%s' has transitions:\n", stateID)
			for event := range stateNode.On {
				fmt.Printf("  - %s\n", event)
			}
			break
		}
	}

	// Show complete machine structure
	machineJSON, _ := json.MarshalIndent(machine, "", "  ")
	fmt.Printf("\nComplete Switch machine:\n%s\n", string(machineJSON))

	// Output: Switch demo showing multi-way branching
}
