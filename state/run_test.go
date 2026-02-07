package state

import (
	"context"
	"testing"
)

func TestRunIsUnnecessary(t *testing.T) {
	ctx := context.Background()

	var executed []string

	// Create a simple pipeline
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			executed = append(executed, "stage1")
		}),
		Action(func(ctx context.Context) {
			executed = append(executed, "stage2")
		}),
		Action(func(ctx context.Context) {
			executed = append(executed, "stage3")
		}),
	)

	// Test 1: Using Run
	executed = nil
	Run(ctx, pipeline)

	expected := []string{"stage1", "stage2", "stage3"}
	if len(executed) != len(expected) {
		t.Fatalf("Run: Expected %d stages, got %d: %v", len(expected), len(executed), executed)
	}
	for i, exp := range expected {
		if executed[i] != exp {
			t.Errorf("Run: Expected executed[%d] = %q, got %q", i, exp, executed[i])
		}
	}

	// Test 2: Directly calling Stage().Next() without Run loop
	executed = nil
	stage := pipeline.Stage()
	result := stage.Next(ctx)

	// In continuation-passing style, calling Next() once should execute the entire pipeline
	if result != nil {
		t.Errorf("Expected pipeline to complete (return nil), but got %v", result)
	}

	if len(executed) != len(expected) {
		t.Fatalf("Direct Next(): Expected %d stages, got %d: %v", len(expected), len(executed), executed)
	}
	for i, exp := range expected {
		if executed[i] != exp {
			t.Errorf("Direct Next(): Expected executed[%d] = %q, got %q", i, exp, executed[i])
		}
	}
}

func TestRunWithComplexPipeline(t *testing.T) {
	ctx := context.WithValue(context.Background(), "condition", true)

	var executed []string

	// Create a complex pipeline with branching
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			executed = append(executed, "setup")
		}),
		Decision(
			func(ctx context.Context) bool {
				return ctx.Value("condition").(bool)
			},
			Action(func(ctx context.Context) {
				executed = append(executed, "true-branch")
			}),
			Action(func(ctx context.Context) {
				executed = append(executed, "false-branch")
			}),
		),
		Action(func(ctx context.Context) {
			executed = append(executed, "cleanup")
		}),
	)

	// Test direct execution without Run loop
	stage := pipeline.Stage()
	result := stage.Next(ctx)

	// Should complete entirely in one call
	if result != nil {
		t.Errorf("Expected pipeline to complete, but got continuation: %v", result)
	}

	expected := []string{"setup", "true-branch", "cleanup"}
	if len(executed) != len(expected) {
		t.Fatalf("Expected %d stages, got %d: %v", len(expected), len(executed), executed)
	}
	for i, exp := range expected {
		if executed[i] != exp {
			t.Errorf("Expected executed[%d] = %q, got %q", i, exp, executed[i])
		}
	}
}

func TestRunWithContextThreading(t *testing.T) {
	ctx := context.Background()

	var values []string

	addValue := func(key, value string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				newCtx := context.WithValue(ctx, key, value)
				values = append(values, "added-"+value)
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	readValue := func(key string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				if val := ctx.Value(key); val != nil {
					values = append(values, "read-"+val.(string))
				} else {
					values = append(values, "read-empty")
				}
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		}
	}

	pipeline := Sequence(
		readValue("key"),
		addValue("key", "first"),
		readValue("key"),
		addValue("key", "second"),
		readValue("key"),
	)

	// Execute directly without Run loop
	stage := pipeline.Stage()
	result := stage.Next(ctx)

	if result != nil {
		t.Errorf("Expected pipeline to complete, but got continuation: %v", result)
	}

	expected := []string{
		"read-empty",
		"added-first",
		"read-first",
		"added-second",
		"read-second",
	}

	if len(values) != len(expected) {
		t.Fatalf("Expected %d values, got %d: %v", len(expected), len(values), values)
	}
	for i, exp := range expected {
		if values[i] != exp {
			t.Errorf("Expected values[%d] = %q, got %q", i, exp, values[i])
		}
	}
}

func TestSingleStageExecution(t *testing.T) {
	ctx := context.Background()

	var executed bool

	stage := Action(func(ctx context.Context) {
		executed = true
	}).Stage()

	// Single stage should complete without Run loop
	result := stage.Next(ctx)

	if result != nil {
		t.Errorf("Expected single stage to complete, but got continuation: %v", result)
	}

	if !executed {
		t.Error("Expected stage to execute")
	}
}

func BenchmarkDirectExecution(b *testing.B) {
	ctx := context.Background()

	pipeline := Sequence(
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stage := pipeline.Stage()
		stage.Next(ctx)
	}
}

func BenchmarkRun(b *testing.B) {
	ctx := context.Background()

	pipeline := Sequence(
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
		Action(func(ctx context.Context) {}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Run(ctx, pipeline)
	}
}
