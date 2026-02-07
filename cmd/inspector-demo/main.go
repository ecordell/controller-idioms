// Package main provides a complete demo of the real-time state machine inspector
// showing how to integrate Stately Inspector with Go controller state machines.
//
// Run this demo and visit http://localhost:8080 to see live state machine
// visualization in your browser using the Stately Inspector.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/authzed/controller-idioms/inspector"
	"github.com/authzed/controller-idioms/state"
)

func main() {
	fmt.Println("🚀 Real-Time State Machine Inspector Demo")
	fmt.Println("==========================================")

	// Create a comprehensive controller state machine
	controllerPipeline := buildControllerStateMachine()

	// Wrap with real-time inspection
	fmt.Println("📊 Setting up real-time inspection...")
	rtInspector, instrumentedPipeline := inspector.WithRealTimeInspection("demo-controller", controllerPipeline)

	// Start the web server
	fmt.Println("🌐 Starting inspector web server...")
	go func() {
		log.Fatal(rtInspector.StartServer(":8080"))
	}()

	// Give the server time to start
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n✨ Demo Ready!")
	fmt.Println("🔗 Open your browser to: http://localhost:8080")
	fmt.Println("📊 You'll see live state machine visualization using Stately Inspector")
	fmt.Println("⚡ State transitions will appear in real-time as the demo runs")

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run demo scenarios
	go runDemoScenarios(instrumentedPipeline)

	fmt.Println("\n🎮 Demo running... Press Ctrl+C to stop")
	<-sigChan

	fmt.Println("\n👋 Demo stopped. Thanks for trying the Real-Time Inspector!")
}

// buildControllerStateMachine creates a realistic controller state machine
// with multiple paths, decisions, and parallel operations
func buildControllerStateMachine() state.NewStage {
	return state.Sequence(
		// Initialization phase
		state.Action(func(ctx context.Context) {
			fmt.Println("🔧 [INIT] Setting finalizer")
			time.Sleep(100 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			fmt.Println("⏸️  [INIT] Checking pause annotation")
			time.Sleep(50 * time.Millisecond)
		}),

		// Resource validation and decision making
		state.Decision(
			func(ctx context.Context) bool {
				resourceExists := ctx.Value("resourceExists").(bool)
				fmt.Printf("🔍 [VALIDATION] Resource exists: %t\n", resourceExists)
				return resourceExists
			},
			// UPDATE PATH: Resource exists
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("✏️  [UPDATE] Validating existing resource")
					time.Sleep(150 * time.Millisecond)
				}),
				state.Decision(
					func(ctx context.Context) bool {
						needsUpdate := ctx.Value("needsUpdate").(bool)
						fmt.Printf("🔄 [UPDATE] Needs update: %t\n", needsUpdate)
						return needsUpdate
					},
					// Resource needs updating
					state.Parallel(
						state.Action(func(ctx context.Context) {
							fmt.Println("📦 [UPDATE] Updating deployment")
							time.Sleep(300 * time.Millisecond)
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("🌐 [UPDATE] Updating service")
							time.Sleep(200 * time.Millisecond)
						}),
						state.Action(func(ctx context.Context) {
							fmt.Println("⚙️  [UPDATE] Updating configmap")
							time.Sleep(250 * time.Millisecond)
						}),
					),
					// No update needed
					state.Action(func(ctx context.Context) {
						fmt.Println("✅ [UPDATE] Resource is up to date")
						time.Sleep(50 * time.Millisecond)
					}),
				),
			),
			// CREATE PATH: Resource doesn't exist
			state.Sequence(
				state.Action(func(ctx context.Context) {
					fmt.Println("➕ [CREATE] Preparing to create resources")
					time.Sleep(100 * time.Millisecond)
				}),
				state.Switch(
					func(ctx context.Context) string {
						return ctx.Value("resourceType").(string)
					},
					map[string]state.NewStage{
						"webapp": state.Sequence(
							state.Action(func(ctx context.Context) {
								fmt.Println("🌍 [CREATE] Creating web application")
								time.Sleep(200 * time.Millisecond)
							}),
							state.Parallel(
								state.Action(func(ctx context.Context) {
									fmt.Println("📦 [CREATE] Creating frontend deployment")
									time.Sleep(400 * time.Millisecond)
								}),
								state.Action(func(ctx context.Context) {
									fmt.Println("🔧 [CREATE] Creating backend deployment")
									time.Sleep(350 * time.Millisecond)
								}),
								state.Action(func(ctx context.Context) {
									fmt.Println("💾 [CREATE] Creating database")
									time.Sleep(500 * time.Millisecond)
								}),
							),
						),
						"microservice": state.Action(func(ctx context.Context) {
							fmt.Println("⚡ [CREATE] Creating microservice")
							time.Sleep(250 * time.Millisecond)
						}),
						"batch": state.Action(func(ctx context.Context) {
							fmt.Println("📊 [CREATE] Creating batch job")
							time.Sleep(300 * time.Millisecond)
						}),
					},
					// Default case
					state.Action(func(ctx context.Context) {
						fmt.Println("❓ [CREATE] Unknown resource type - using default")
						time.Sleep(150 * time.Millisecond)
					}),
				),
			),
		),

		// Status update and finalization
		state.Action(func(ctx context.Context) {
			fmt.Println("📊 [STATUS] Updating resource status")
			time.Sleep(100 * time.Millisecond)
		}),
		state.Action(func(ctx context.Context) {
			fmt.Println("🎯 [COMPLETE] Controller reconciliation finished")
			time.Sleep(50 * time.Millisecond)
		}),
	)
}

// runDemoScenarios executes various scenarios to demonstrate the inspector
func runDemoScenarios(pipeline state.NewStage) {
	scenarios := []struct {
		name           string
		resourceExists bool
		needsUpdate    bool
		resourceType   string
		description    string
	}{
		{
			"New WebApp Creation",
			false, false, "webapp",
			"Creating a new web application with frontend, backend, and database",
		},
		{
			"Microservice Update",
			true, true, "microservice",
			"Updating an existing microservice deployment",
		},
		{
			"Batch Job Creation",
			false, false, "batch",
			"Creating a new batch processing job",
		},
		{
			"No Changes Needed",
			true, false, "webapp",
			"Resource exists and is up to date - no changes needed",
		},
		{
			"Unknown Resource Type",
			false, false, "mystery",
			"Handling an unknown resource type with default behavior",
		},
	}

	for i, scenario := range scenarios {
		time.Sleep(2 * time.Second) // Pause between scenarios

		fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Printf("📋 Scenario %d/%d: %s\n", i+1, len(scenarios), scenario.name)
		fmt.Printf("💭 %s\n", scenario.description)
		fmt.Printf(strings.Repeat("=", 60) + "\n")

		// Create context with scenario parameters
		ctx := context.Background()
		ctx = context.WithValue(ctx, "resourceExists", scenario.resourceExists)
		ctx = context.WithValue(ctx, "needsUpdate", scenario.needsUpdate)
		ctx = context.WithValue(ctx, "resourceType", scenario.resourceType)

		// Execute the pipeline
		state.RunNewStage(ctx, pipeline)

		fmt.Printf("✅ Scenario completed\n")
	}

	// Run a few more cycles to show continuous operation
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Println("🔄 Running continuous operations (watch the inspector!)...")
	fmt.Printf(strings.Repeat("=", 60) + "\n")

	for i := 0; i < 5; i++ {
		time.Sleep(3 * time.Second)

		// Alternate between different scenarios
		resourceExists := i%2 == 0
		needsUpdate := i%3 == 0
		resourceTypes := []string{"webapp", "microservice", "batch"}
		resourceType := resourceTypes[i%len(resourceTypes)]

		fmt.Printf("\n🔄 Continuous operation #%d (type: %s)\n", i+1, resourceType)

		ctx := context.Background()
		ctx = context.WithValue(ctx, "resourceExists", resourceExists)
		ctx = context.WithValue(ctx, "needsUpdate", needsUpdate)
		ctx = context.WithValue(ctx, "resourceType", resourceType)

		state.RunNewStage(ctx, pipeline)
	}

	fmt.Println("\n🏁 Demo scenarios complete!")
	fmt.Println("💡 The inspector will continue running - try refreshing the browser")
	fmt.Println("🔄 You can restart this demo anytime to see more transitions")
}
