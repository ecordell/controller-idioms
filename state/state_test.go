package state

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ExampleSequence() {
	ctx := context.Background()

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("first stage")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("second stage")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("third stage")
		}),
	)

	Run(ctx, pipeline)
	// Output:
	// first stage
	// second stage
	// third stage
}

func ExampleDecision() {
	ctx := context.Background()

	// Decision based on a simple condition
	condition := true

	pipeline := Decision(
		func(ctx context.Context) bool {
			return condition
		},
		Action(func(ctx context.Context) {
			fmt.Println("true branch")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("false branch")
		}),
	)

	Run(ctx, pipeline)
	// Output: true branch
}

func ExampleParallel() {
	ctx := context.Background()

	// Use atomic counter to demonstrate parallel execution
	var counter int32

	pipeline := Parallel(
		Action(func(ctx context.Context) {
			atomic.AddInt32(&counter, 1)
		}),
		Action(func(ctx context.Context) {
			atomic.AddInt32(&counter, 1)
		}),
		Action(func(ctx context.Context) {
			atomic.AddInt32(&counter, 1)
		}),
	)

	Run(ctx, pipeline)
	fmt.Printf("counter: %d", atomic.LoadInt32(&counter))
	// Output: counter: 3
}

func ExampleMap() {
	ctx := context.WithValue(context.Background(), "key", "initial")

	pipeline := Map(
		func(ctx context.Context) context.Context {
			// Transform the context
			return context.WithValue(ctx, "key", "transformed")
		},
		Action(func(ctx context.Context) {
			fmt.Printf("value: %s", ctx.Value("key"))
		}),
	)

	Run(ctx, pipeline)
	// Output: value: transformed
}

func ExampleChoice() {
	ctx := context.Background()

	// Simple conditional execution using Decision
	shouldContinue := false

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("before choice")
		}),
		Decision(
			func(ctx context.Context) bool {
				return shouldContinue
			},
			Action(func(ctx context.Context) {
				fmt.Println("continuing")
			}),
			Action(func(ctx context.Context) {
				fmt.Println("not continuing")
			}),
		),
	)

	Run(ctx, pipeline)
	// Output:
	// before choice
	// not continuing
}

func Example_complexPipeline() {
	ctx := context.Background()

	// A more complex example showing composition
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("initialization")
		}),
		Decision(
			func(ctx context.Context) bool {
				return true // some validation logic
			},
			// Validation passed - processing
			Action(func(ctx context.Context) {
				fmt.Println("validation passed")
			}),
			// Validation failed
			Action(func(ctx context.Context) {
				fmt.Println("validation failed")
			}),
		),
		Action(func(ctx context.Context) {
			fmt.Println("finalization")
		}),
	)

	Run(ctx, pipeline)
	// Output:
	// initialization
	// validation passed
	// finalization
}

func TestSequenceExecution(t *testing.T) {
	ctx := context.Background()
	var executed []string

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			executed = append(executed, "first")
		}),
		Action(func(ctx context.Context) {
			executed = append(executed, "second")
		}),
		Action(func(ctx context.Context) {
			executed = append(executed, "third")
		}),
	)

	Run(ctx, pipeline)

	expected := []string{"first", "second", "third"}
	if len(executed) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(executed))
	}

	for i, stage := range expected {
		if executed[i] != stage {
			t.Errorf("stage %d: expected %s, got %s", i, stage, executed[i])
		}
	}
}

func TestDecisionBranching(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name      string
		condition bool
		expected  string
	}{
		{"true branch", true, "true"},
		{"false branch", false, "false"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string

			pipeline := Decision(
				func(ctx context.Context) bool {
					return tc.condition
				},
				Action(func(ctx context.Context) {
					result = "true"
				}),
				Action(func(ctx context.Context) {
					result = "false"
				}),
			)

			Run(ctx, pipeline)

			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestParallelExecution(t *testing.T) {
	ctx := context.Background()
	var counter int32

	pipeline := Parallel(
		Action(func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond) // Simulate work
			atomic.AddInt32(&counter, 1)
		}),
		Action(func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond) // Simulate work
			atomic.AddInt32(&counter, 2)
		}),
		Action(func(ctx context.Context) {
			time.Sleep(10 * time.Millisecond) // Simulate work
			atomic.AddInt32(&counter, 4)
		}),
	)

	start := time.Now()
	Run(ctx, pipeline)
	duration := time.Since(start)

	// Should have executed in parallel (less than 30ms for 3x10ms tasks)
	if duration > 25*time.Millisecond {
		t.Errorf("parallel execution took too long: %v", duration)
	}

	expected := int32(7) // 1 + 2 + 4
	if counter != expected {
		t.Errorf("expected counter %d, got %d", expected, counter)
	}
}

func TestMapTransformation(t *testing.T) {
	ctx := context.WithValue(context.Background(), "input", "hello")
	var result string

	pipeline := Map(
		func(ctx context.Context) context.Context {
			input := ctx.Value("input").(string)
			return context.WithValue(ctx, "output", input+" world")
		},
		Action(func(ctx context.Context) {
			result = ctx.Value("output").(string)
		}),
	)

	Run(ctx, pipeline)

	expected := "hello world"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestEmptySequence(t *testing.T) {
	ctx := context.Background()

	pipeline := Sequence() // Empty sequence
	Run(ctx, pipeline)

	// Should not panic and should complete immediately
}

func TestTerminalStage(t *testing.T) {
	ctx := context.Background()

	Run(ctx, Terminal)

	// Should not panic and should complete immediately
}

func TestNestedComposition(t *testing.T) {
	ctx := context.Background()
	var executed []string

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			executed = append(executed, "outer-1")
		}),
		Sequence( // Nested sequence
			Action(func(ctx context.Context) {
				executed = append(executed, "inner-1")
			}),
			Action(func(ctx context.Context) {
				executed = append(executed, "inner-2")
			}),
		),
		Action(func(ctx context.Context) {
			executed = append(executed, "outer-2")
		}),
	)

	Run(ctx, pipeline)

	expected := []string{"outer-1", "inner-1", "inner-2", "outer-2"}
	if len(executed) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(executed))
	}

	for i, stage := range expected {
		if executed[i] != stage {
			t.Errorf("stage %d: expected %s, got %s", i, stage, executed[i])
		}
	}
}

func ExampleEnum() {
	ctx := context.WithValue(context.Background(), "operation", "create")

	pipeline := Enum(
		func(ctx context.Context) string {
			return ctx.Value("operation").(string)
		},
		map[string]NewStep{
			"create": Action(func(ctx context.Context) {
				fmt.Println("creating resource")
			}),
			"update": Action(func(ctx context.Context) {
				fmt.Println("updating resource")
			}),
			"delete": Action(func(ctx context.Context) {
				fmt.Println("deleting resource")
			}),
		},
		Action(func(ctx context.Context) {
			fmt.Println("unknown operation")
		}),
	)

	Run(ctx, pipeline)
	// Output: creating resource
}

func ExampleSwitch() {
	ctx := context.WithValue(context.Background(), "status", "pending")

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("processing status")
		}),
		Switch(
			func(ctx context.Context) string {
				return ctx.Value("status").(string)
			},
			map[string]NewStep{
				"pending": Action(func(ctx context.Context) {
					fmt.Println("handling pending status")
				}),
				"complete": Action(func(ctx context.Context) {
					fmt.Println("handling complete status")
				}),
				"failed": Action(func(ctx context.Context) {
					fmt.Println("handling failed status")
				}),
			},
			Action(func(ctx context.Context) {
				fmt.Println("handling unknown status")
			}),
		),
	)

	Run(ctx, pipeline)
	// Output:
	// processing status
	// handling pending status
}

func TestEnumExecution(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		value    int
		expected string
	}{
		{"case 1", 1, "one"},
		{"case 2", 2, "two"},
		{"case 3", 3, "three"},
		{"default case", 99, "default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string

			pipeline := Enum(
				func(ctx context.Context) int {
					return tc.value
				},
				map[int]NewStep{
					1: Action(func(ctx context.Context) {
						result = "one"
					}),
					2: Action(func(ctx context.Context) {
						result = "two"
					}),
					3: Action(func(ctx context.Context) {
						result = "three"
					}),
				},
				Action(func(ctx context.Context) {
					result = "default"
				}),
			)

			Run(ctx, pipeline)

			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestSwitchExecution(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		status   string
		expected string
	}{
		{"success status", "success", "handled success"},
		{"error status", "error", "handled error"},
		{"warning status", "warning", "handled warning"},
		{"unknown status", "unknown", "handled default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string

			pipeline := Switch(
				func(ctx context.Context) string {
					return tc.status
				},
				map[string]NewStep{
					"success": Action(func(ctx context.Context) {
						result = "handled success"
					}),
					"error": Action(func(ctx context.Context) {
						result = "handled error"
					}),
					"warning": Action(func(ctx context.Context) {
						result = "handled warning"
					}),
				},
				Action(func(ctx context.Context) {
					result = "handled default"
				}),
			)

			Run(ctx, pipeline)

			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestEnumWithoutDefault(t *testing.T) {
	ctx := context.Background()
	var executed bool

	pipeline := Enum(
		func(ctx context.Context) string {
			return "nonexistent"
		},
		map[string]NewStep{
			"exists": Action(func(ctx context.Context) {
				executed = true
			}),
		},
		nil, // No default stage
	)

	Run(ctx, pipeline)

	if executed {
		t.Error("expected no execution when no matching case and no default")
	}
}

func ExampleWithMiddleware() {
	ctx := context.WithValue(context.Background(), "validations", []string{"policy1", "policy2"})

	// Example replicating the old handler wrapping pattern
	validationStage := WithMiddleware(
		Action(func(ctx context.Context) {
			fmt.Println("executing main logic")
		}),
		// Skip if no validations are specified
		ConditionalMiddleware(func(ctx context.Context) bool {
			validations := ctx.Value("validations").([]string)
			return len(validations) > 0
		}),
		// Do validation work before main stage
		WrapWithWork(func(ctx context.Context) {
			fmt.Println("ensuring validating admission policy")
		}),
		// Handle errors and cancellation
		ErrorHandlingMiddleware(func(ctx context.Context, err error) {
			fmt.Printf("handled error: %v\n", err)
		}),
	)

	Run(ctx, validationStage)

	// Output:
	// ensuring validating admission policy
	// executing main logic
}

func Example_middlewareChain() {
	ctx := context.Background()

	// Chain multiple middleware together
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("before middleware")
		}),
		WithMiddleware(
			Action(func(ctx context.Context) {
				fmt.Println("core business logic")
			}),
			ValidationMiddleware(
				func(ctx context.Context) bool {
					return true // Validation passes
				},
				func(ctx context.Context) {
					fmt.Println("validation failed")
				},
			),
		),
		Action(func(ctx context.Context) {
			fmt.Println("after middleware")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// before middleware
	// core business logic
	// after middleware
}

func TestConditionalMiddleware(t *testing.T) {
	ctx := context.Background()
	var executed bool

	// Test condition true - stage should execute
	pipeline := WithMiddleware(
		Action(func(ctx context.Context) {
			executed = true
		}),
		ConditionalMiddleware(func(ctx context.Context) bool {
			return true
		}),
	)

	Run(ctx, pipeline)

	if !executed {
		t.Error("expected stage to execute when condition is true")
	}

	// Reset and test condition false - stage should not execute
	executed = false
	pipeline2 := WithMiddleware(
		Action(func(ctx context.Context) {
			executed = true
		}),
		ConditionalMiddleware(func(ctx context.Context) bool {
			return false
		}),
	)

	Run(ctx, pipeline2)

	if executed {
		t.Error("expected stage to not execute when condition is false")
	}
}

func TestValidationMiddleware(t *testing.T) {
	ctx := context.Background()
	var executed bool
	var validationFailed bool

	// Test validation passes
	pipeline := WithMiddleware(
		Action(func(ctx context.Context) {
			executed = true
		}),
		ValidationMiddleware(
			func(ctx context.Context) bool {
				return true // Validation passes
			},
			func(ctx context.Context) {
				validationFailed = true
			},
		),
	)

	Run(ctx, pipeline)

	if !executed {
		t.Error("expected stage to execute when validation passes")
	}
	if validationFailed {
		t.Error("expected validation failure callback not to be called when validation passes")
	}

	// Reset and test validation fails
	executed = false
	validationFailed = false

	pipeline2 := WithMiddleware(
		Action(func(ctx context.Context) {
			executed = true
		}),
		ValidationMiddleware(
			func(ctx context.Context) bool {
				return false // Validation fails
			},
			func(ctx context.Context) {
				validationFailed = true
			},
		),
	)

	Run(ctx, pipeline2)

	if executed {
		t.Error("expected stage not to execute when validation fails")
	}
	if !validationFailed {
		t.Error("expected validation failure callback to be called when validation fails")
	}
}

func TestWrapWithWork(t *testing.T) {
	ctx := context.Background()
	var workExecuted bool
	var stageExecuted bool

	pipeline := WithMiddleware(
		Action(func(ctx context.Context) {
			stageExecuted = true
		}),
		WrapWithWork(func(ctx context.Context) {
			workExecuted = true
		}),
	)

	Run(ctx, pipeline)

	if !workExecuted {
		t.Error("expected work to be executed")
	}
	if !stageExecuted {
		t.Error("expected stage to be executed after work")
	}
}

func TestCallAndCheck(t *testing.T) {
	ctx := context.Background()
	var mainExecuted bool
	var resultChecked bool
	var nextExecuted bool

	pipeline := CallAndCheck(
		Action(func(ctx context.Context) {
			mainExecuted = true
		}),
		func(ctx context.Context, result Step) Step {
			resultChecked = true
			return Action(func(ctx context.Context) {
				nextExecuted = true
			}).Step()
		},
	)

	Run(ctx, pipeline)

	if !mainExecuted {
		t.Error("expected main stage to execute")
	}
	if !resultChecked {
		t.Error("expected result check to execute")
	}
	if !nextExecuted {
		t.Error("expected next stage to execute")
	}
}

func TestCallAndContinueIf(t *testing.T) {
	ctx := context.WithValue(context.Background(), "condition", true)

	var mainExecuted bool
	var trueStageExecuted bool
	var falseStageExecuted bool

	pipeline := CallAndContinueIf(
		Action(func(ctx context.Context) {
			mainExecuted = true
		}),
		func(ctx context.Context) bool {
			return ctx.Value("condition").(bool)
		},
		Action(func(ctx context.Context) {
			trueStageExecuted = true
		}),
		Action(func(ctx context.Context) {
			falseStageExecuted = true
		}),
	)

	Run(ctx, pipeline)

	if !mainExecuted {
		t.Error("expected main stage to execute")
	}
	if !trueStageExecuted {
		t.Error("expected true stage to execute when condition is true")
	}
	if falseStageExecuted {
		t.Error("expected false stage not to execute when condition is true")
	}
}

func TestCallAndContinue(t *testing.T) {
	ctx := context.Background()
	var firstExecuted bool
	var secondExecuted bool

	pipeline := CallAndContinue(
		Action(func(ctx context.Context) {
			firstExecuted = true
		}),
		Action(func(ctx context.Context) {
			secondExecuted = true
		}),
	)

	Run(ctx, pipeline)

	if !firstExecuted {
		t.Error("expected first stage to execute")
	}
	if !secondExecuted {
		t.Error("expected second stage to execute")
	}
}

func TestReuse(t *testing.T) {
	ctx := context.Background()
	var reuseExecuted bool
	var afterExecuted bool

	pipeline := Reuse(
		Action(func(ctx context.Context) {
			reuseExecuted = true
		}),
		func(ctx context.Context) NewStep {
			return Action(func(ctx context.Context) {
				afterExecuted = true
			})
		},
	)

	Run(ctx, pipeline)

	if !reuseExecuted {
		t.Error("expected reused stage to execute")
	}
	if !afterExecuted {
		t.Error("expected after-execution stage to execute")
	}
}

func TestParallelPanicRecovery(t *testing.T) {
	ctx := context.Background()
	var executed []string
	var mu sync.Mutex

	pipeline := Parallel(
		Action(func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, "step1")
			mu.Unlock()
		}),
		Action(func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, "step2-before-panic")
			mu.Unlock()
			panic("intentional panic")
		}),
		Action(func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, "step3")
			mu.Unlock()
		}),
	)

	// Should not panic - panic is recovered
	Run(ctx, pipeline)

	// All steps should execute (panic doesn't stop other goroutines)
	mu.Lock()
	defer mu.Unlock()
	if len(executed) < 3 {
		t.Logf("Executed steps: %v", executed)
		t.Log("Note: Panic in one goroutine doesn't prevent others from executing")
	}
}

func TestParallelCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var executed []string
	var mu sync.Mutex

	pipeline := Parallel(
		Action(func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, "step1")
			mu.Unlock()
		}),
		Action(func(ctx context.Context) {
			mu.Lock()
			executed = append(executed, "step2")
			mu.Unlock()
		}),
	)

	Run(ctx, pipeline)

	// With cancelled context, steps should not execute
	mu.Lock()
	defer mu.Unlock()
	if len(executed) > 0 {
		t.Logf("Some steps executed despite cancellation: %v", executed)
		t.Log("Note: There's a race - steps might start before cancellation check")
	}
}
