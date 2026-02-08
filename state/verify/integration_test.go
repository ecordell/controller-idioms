package verify_test

import (
	"context"
	"testing"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/verify"
)

func TestIntegrationCompleteVerification(t *testing.T) {
	// Create a realistic pipeline
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			time.Sleep(1 * time.Millisecond)
		}),
		state.Decision(
			func(ctx context.Context) bool { return true },
			state.Action(func(ctx context.Context) {
				time.Sleep(1 * time.Millisecond)
			}),
			state.Action(func(ctx context.Context) {}),
		),
		state.Action(func(ctx context.Context) {
			time.Sleep(1 * time.Millisecond)
		}),
	)

	// Test multiple verifications
	t.Run("NoPanic", func(t *testing.T) {
		if err := verify.VerifyNoPanic(pipeline); err != nil {
			t.Errorf("VerifyNoPanic failed: %v", err)
		}
	})

	t.Run("Terminates", func(t *testing.T) {
		if err := verify.VerifyTerminates(pipeline, 1*time.Second); err != nil {
			t.Errorf("VerifyTerminates failed: %v", err)
		}
	})

	t.Run("Progress", func(t *testing.T) {
		if err := verify.VerifyProgress(pipeline, 10); err != nil {
			t.Errorf("VerifyProgress failed: %v", err)
		}
	})

	// Test trace analysis
	t.Run("Trace", func(t *testing.T) {
		tracer, trace := verify.NewTracer()
		wrapped := state.WithMiddleware(pipeline, tracer)
		state.Run(context.Background(), wrapped)

		if len(trace.Steps) < 3 {
			t.Errorf("expected at least 3 steps, got %d", len(trace.Steps))
		}

		// Verify all steps have durations
		for i, step := range trace.Steps {
			if step.Duration == 0 {
				t.Errorf("step %d has zero duration", i)
			}
		}
	})
}
