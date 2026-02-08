package verify_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/verify"
)

func TestExecutionTrace(t *testing.T) {
	tracer, trace := verify.NewTracer()

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	wrapped := state.WithMiddleware(pipeline, tracer)
	state.Run(context.Background(), wrapped)

	if len(trace.Steps) < 3 {
		t.Errorf("expected at least 3 steps traced, got %d", len(trace.Steps))
	}
}

func TestExecutionTraceWithDecision(t *testing.T) {
	tracer, trace := verify.NewTracer()

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {}),
			state.Action(func(ctx context.Context) {}),
		),
	)

	wrapped := state.WithMiddleware(pipeline, tracer)
	state.Run(context.Background(), wrapped)

	if len(trace.Steps) < 2 {
		t.Errorf("expected at least 2 steps traced, got %d", len(trace.Steps))
	}
}
