package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestWrapStep(t *testing.T) {
	executed := false
	step := state.StepFunc(func(ctx context.Context) state.Step {
		executed = true
		return nil
	})

	// Wrap the step with a no-op wrapper
	wrapper := func(s state.NewStep) state.NewStep {
		return s
	}

	wrapped := transform.WrapStep(step, wrapper)
	wrapped.Run(context.Background())

	if !executed {
		t.Error("wrapped step was not executed")
	}
}

func TestWrapStepNil(t *testing.T) {
	wrapper := func(s state.NewStep) state.NewStep {
		return s
	}

	wrapped := transform.WrapStep(nil, wrapper)
	if wrapped != nil {
		t.Error("wrapping nil step should return nil")
	}
}

func TestRecursiveMiddleware(t *testing.T) {
	var beforeCount, afterCount int
	var executionOrder []string

	before := func(ctx context.Context) {
		beforeCount++
		executionOrder = append(executionOrder, "before")
	}
	after := func(ctx context.Context) {
		afterCount++
		executionOrder = append(executionOrder, "after")
	}

	middleware := transform.RecursiveMiddleware(before, after)

	// Create a simple sequence
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) { executionOrder = append(executionOrder, "A") }),
		state.Action(func(ctx context.Context) { executionOrder = append(executionOrder, "B") }),
		state.Action(func(ctx context.Context) { executionOrder = append(executionOrder, "C") }),
	)

	// Apply middleware and run
	wrapped := state.WithMiddleware(pipeline, middleware)
	state.Run(context.Background(), wrapped)

	t.Logf("Execution order: %v", executionOrder)

	// Should have called before/after for each step
	if beforeCount != 3 {
		t.Errorf("expected 3 before calls, got %d", beforeCount)
	}
	if afterCount != 3 {
		t.Errorf("expected 3 after calls, got %d", afterCount)
	}
}

func TestRecursiveMiddlewareWithDecision(t *testing.T) {
	var executionOrder []string

	before := func(ctx context.Context) {
		executionOrder = append(executionOrder, "before")
	}
	after := func(ctx context.Context) {
		executionOrder = append(executionOrder, "after")
	}

	middleware := transform.RecursiveMiddleware(before, after)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step1")
		}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {
				executionOrder = append(executionOrder, "true-branch")
			}),
			state.Action(func(ctx context.Context) {
				executionOrder = append(executionOrder, "false-branch")
			}),
		),
		state.Action(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step3")
		}),
	)

	wrapped := state.WithMiddleware(pipeline, middleware)
	state.Run(context.Background(), wrapped)

	// Verify before/after wraps each step
	expected := []string{
		"before", "step1", "after",
		"before", "before", "true-branch", "after", "after",
		"before", "step3", "after",
	}

	if len(executionOrder) != len(expected) {
		t.Errorf("expected %d calls, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}
}
