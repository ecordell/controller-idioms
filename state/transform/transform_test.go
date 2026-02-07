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
