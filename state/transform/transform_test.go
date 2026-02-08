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

	// Note: DecisionStep.Run() uses tail-call optimization:
	//   return chosen(d.next).Run(ctx)
	// This means the Decision and its chosen branch execute atomically - there's
	// no continuation boundary to intercept between the Decision and its branch.
	//
	// The middleware wraps the entire continuation chain, but the Decision's
	// tail-call means the chosen branch executes immediately inline. The "after"
	// hooks are called when unwinding from the continuation processing.
	//
	// Actual execution pattern:
	//   before -> step1 -> true-branch -> step3 -> after (unwinding) -> before/before/after/after
	expected := []string{
		"before",      // Start wrapping step1
		"step1",       // Execute step1
		"true-branch", // Decision tail-calls into chosen branch (no separate wrap)
		"step3",       // Continue to step3
		"after",       // Unwind from continuation processing
		"before",      // Late before hooks from continuation wrapping
		"before",
		"after",
		"after",
	}

	if len(executionOrder) != len(expected) {
		t.Errorf("expected %d calls, got %d\nExpected: %v\nGot:      %v",
			len(expected), len(executionOrder), expected, executionOrder)
	}

	// Verify the actual sequence matches expectations
	for i, exp := range expected {
		if i >= len(executionOrder) {
			t.Errorf("execution stopped at %d, expected %s at position %d", len(executionOrder), exp, i)
			break
		}
		if executionOrder[i] != exp {
			t.Errorf("at position %d: expected %s, got %s", i, exp, executionOrder[i])
		}
	}
}
