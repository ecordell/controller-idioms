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
		if err := verify.VerifyProgress(pipeline, 20); err != nil {
			t.Errorf("VerifyProgress failed: %v", err)
		}
	})

	// Test trace analysis
	t.Run("Trace", func(t *testing.T) {
		tracer, trace := verify.NewTracer()
		wrapped := state.WithMiddleware(pipeline, tracer)
		state.Run(context.Background(), wrapped)

		// Verify we captured some execution steps
		if len(trace.Steps) == 0 {
			t.Error("expected trace to capture steps, got 0")
		}

		// Verify steps have start times
		for i, step := range trace.Steps {
			if step.StartTime.IsZero() {
				t.Errorf("step %d has zero start time", i)
			}
		}
	})
}

func TestIntegrationWithCombinedVerifications(t *testing.T) {
	// Test verifying multiple properties on different pipelines
	t.Run("SafePipeline", func(t *testing.T) {
		safe := state.Sequence(
			state.Action(func(ctx context.Context) {}),
			state.Action(func(ctx context.Context) {}),
		)

		// Should pass all verifications
		if err := verify.VerifyNoPanic(safe); err != nil {
			t.Errorf("safe pipeline failed VerifyNoPanic: %v", err)
		}
		if err := verify.VerifyTerminates(safe, 100*time.Millisecond); err != nil {
			t.Errorf("safe pipeline failed VerifyTerminates: %v", err)
		}
		if err := verify.VerifyProgress(safe, 10); err != nil {
			t.Errorf("safe pipeline failed VerifyProgress: %v", err)
		}
	})

	t.Run("PanickyPipeline", func(t *testing.T) {
		panicky := state.Action(func(ctx context.Context) {
			panic("intentional panic")
		})

		// Should detect panic
		if err := verify.VerifyNoPanic(panicky); err == nil {
			t.Error("expected VerifyNoPanic to detect panic")
		}
	})

	t.Run("SlowPipeline", func(t *testing.T) {
		slow := state.Action(func(ctx context.Context) {
			time.Sleep(50 * time.Millisecond)
		})

		// Should timeout
		if err := verify.VerifyTerminates(slow, 10*time.Millisecond); err == nil {
			t.Error("expected VerifyTerminates to detect timeout")
		}
	})
}
