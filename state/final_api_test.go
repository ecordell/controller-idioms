package state

import (
	"context"
	"testing"
)

func TestFinalAPIDemo(t *testing.T) {
	ctx := context.Background()

	var executed []string
	var contextValues []string

	// Helper to add context value and continue
	addContextValue := func(key, value string) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				executed = append(executed, "adding-"+value)
				newCtx := context.WithValue(ctx, key, value)
				if next != nil {
					return next.Run(newCtx)
				}
				return nil
			})
		}
	}

	// Helper to read context value
	readContextValue := func(key string) NewStep {
		return Action(func(ctx context.Context) {
			if val := ctx.Value(key); val != nil {
				contextValues = append(contextValues, val.(string))
				executed = append(executed, "reading-"+val.(string))
			} else {
				executed = append(executed, "reading-empty")
			}
		})
	}

	// Complex pipeline demonstrating all features
	pipeline := Sequence(
		// 1. Start with empty context
		readContextValue("key"),

		// 2. Add a value
		addContextValue("key", "first"),

		// 3. Read the value (should see "first")
		readContextValue("key"),

		// 4. Conditional branching based on context
		Decision(
			func(ctx context.Context) bool {
				val := ctx.Value("key")
				return val != nil && val.(string) == "first"
			},
			// True branch: modify context again
			Sequence(
				addContextValue("key", "modified"),
				readContextValue("key"), // Should see "modified"
			),
			// False branch: shouldn't execute
			Action(func(ctx context.Context) {
				executed = append(executed, "false-branch")
			}),
		),

		// 5. Multi-way branching with Enum
		Enum(
			func(ctx context.Context) string {
				if val := ctx.Value("key"); val != nil {
					return val.(string)
				}
				return "unknown"
			},
			map[string]NewStep{
				"modified": Sequence(
					Action(func(ctx context.Context) {
						executed = append(executed, "enum-modified")
					}),
					addContextValue("key", "final"),
				),
				"other": Action(func(ctx context.Context) {
					executed = append(executed, "enum-other")
				}),
			},
			Action(func(ctx context.Context) {
				executed = append(executed, "enum-default")
			}),
		),

		// 6. Final read
		readContextValue("key"), // Should see "final"

		// 7. Parallel execution (context preserved in each branch)
		Parallel(
			Action(func(ctx context.Context) {
				if val := ctx.Value("key"); val != nil {
					executed = append(executed, "parallel1-"+val.(string))
				}
			}),
			Action(func(ctx context.Context) {
				if val := ctx.Value("key"); val != nil {
					executed = append(executed, "parallel2-"+val.(string))
				}
			}),
		),

		// 8. Final cleanup
		Action(func(ctx context.Context) {
			executed = append(executed, "cleanup")
		}),
	)

	// Execute the entire pipeline with one simple call
	Run(ctx, pipeline)

	// Verify execution order (except parallel parts which are non-deterministic)
	expectedBeforeParallel := []string{
		"reading-empty",    // 1. Started with empty context
		"adding-first",     // 2. Added "first"
		"reading-first",    // 3. Read "first"
		"adding-modified",  // 4. Decision true branch: added "modified"
		"reading-modified", // 4. Decision true branch: read "modified"
		"enum-modified",    // 5. Enum matched "modified"
		"adding-final",     // 5. Enum branch: added "final"
		"reading-final",    // 6. Read "final"
	}

	expectedContextValues := []string{
		"first",    // First read after adding "first"
		"modified", // Read after decision branch modified it
		"final",    // Read after enum branch set final value
	}

	// Verify we have the right total number of steps
	expectedTotalSteps := len(expectedBeforeParallel) + 2 + 1 // +2 parallel +1 cleanup
	if len(executed) != expectedTotalSteps {
		t.Fatalf("Expected %d executed steps, got %d: %v", expectedTotalSteps, len(executed), executed)
	}

	// Verify the deterministic sequence before parallel execution
	for i, expected := range expectedBeforeParallel {
		if executed[i] != expected {
			t.Errorf("Expected executed[%d] = %q, got %q", i, expected, executed[i])
		}
	}

	// Verify parallel execution happened (non-deterministic order)
	parallelStart := len(expectedBeforeParallel)
	parallel1Found := false
	parallel2Found := false
	for i := parallelStart; i < parallelStart+2; i++ {
		if executed[i] == "parallel1-final" {
			parallel1Found = true
		} else if executed[i] == "parallel2-final" {
			parallel2Found = true
		} else {
			t.Errorf("Unexpected parallel execution step: %q", executed[i])
		}
	}
	if !parallel1Found {
		t.Error("Expected parallel1-final to execute")
	}
	if !parallel2Found {
		t.Error("Expected parallel2-final to execute")
	}

	// Verify cleanup happened last
	if executed[len(executed)-1] != "cleanup" {
		t.Errorf("Expected cleanup to be last, got %q", executed[len(executed)-1])
	}

	// Verify context values were properly threaded
	if len(contextValues) != len(expectedContextValues) {
		t.Fatalf("Expected %d context values, got %d: %v", len(expectedContextValues), len(contextValues), contextValues)
	}

	for i, expected := range expectedContextValues {
		if contextValues[i] != expected {
			t.Errorf("Expected contextValues[%d] = %q, got %q", i, expected, contextValues[i])
		}
	}
}

func TestSimpleAPIUsage(t *testing.T) {
	ctx := context.Background()

	var result string

	// Simple three-stage pipeline
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			result += "A"
		}),
		Action(func(ctx context.Context) {
			result += "B"
		}),
		Action(func(ctx context.Context) {
			result += "C"
		}),
	)

	// Execute with clean API
	Run(ctx, pipeline)

	if result != "ABC" {
		t.Errorf("Expected result 'ABC', got %q", result)
	}
}

func TestDirectStageExecution(t *testing.T) {
	ctx := context.Background()

	var executed bool

	// You can still execute stages directly without Run()
	stage := Action(func(ctx context.Context) {
		executed = true
	}).Step()

	// Direct execution - also works
	stage.Run(ctx)

	if !executed {
		t.Error("Expected stage to execute")
	}
}

func TestContextThreadingShowcase(t *testing.T) {
	ctx := context.Background()

	var phases []string

	// Showcase that demonstrates the power of context threading
	authenticate := func(user string) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				phases = append(phases, "authenticating-"+user)
				authCtx := context.WithValue(ctx, "user", user)
				authCtx = context.WithValue(authCtx, "authenticated", true)
				if next != nil {
					return next.Run(authCtx)
				}
				return nil
			})
		}
	}

	authorize := func(resource string) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				user := ctx.Value("user").(string)
				phases = append(phases, "authorizing-"+user+"-for-"+resource)
				authzCtx := context.WithValue(ctx, "authorized", resource)
				if next != nil {
					return next.Run(authzCtx)
				}
				return nil
			})
		}
	}

	processRequest := Action(func(ctx context.Context) {
		user := ctx.Value("user").(string)
		resource := ctx.Value("authorized").(string)
		phases = append(phases, "processing-"+resource+"-for-"+user)
	})

	auditLog := Action(func(ctx context.Context) {
		user := ctx.Value("user").(string)
		phases = append(phases, "auditing-"+user)
	})

	// Real-world-like pipeline
	requestPipeline := Sequence(
		authenticate("alice"),
		authorize("database"),
		processRequest,
		auditLog,
	)

	Run(ctx, requestPipeline)

	expected := []string{
		"authenticating-alice",
		"authorizing-alice-for-database",
		"processing-database-for-alice",
		"auditing-alice",
	}

	if len(phases) != len(expected) {
		t.Fatalf("Expected %d phases, got %d: %v", len(expected), len(phases), phases)
	}

	for i, exp := range expected {
		if phases[i] != exp {
			t.Errorf("Expected phases[%d] = %q, got %q", i, exp, phases[i])
		}
	}
}
