package verify

import (
	"context"
	"fmt"
	"time"

	"github.com/authzed/controller-idioms/state"
)

// VerifyNoPanic verifies that the pipeline executes without panicking.
func VerifyNoPanic(pipeline state.NewStep) error {
	tracer, trace := NewTracer()

	// Add panic recovery middleware
	panicDetector := func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			return state.StepFunc(func(ctx context.Context) state.Step {
				defer func() {
					if r := recover(); r != nil {
						// Record panic in trace
						trace.mu.Lock()
						if len(trace.Steps) > 0 {
							trace.Steps[len(trace.Steps)-1].Panicked = true
						}
						trace.mu.Unlock()
					}
				}()
				return step(next).Run(ctx)
			})
		}
	}

	wrapped := state.WithMiddleware(pipeline, tracer, panicDetector)
	state.Run(context.Background(), wrapped)

	// Check if any step panicked
	trace.mu.Lock()
	defer trace.mu.Unlock()

	for _, step := range trace.Steps {
		if step.Panicked {
			return fmt.Errorf("panic detected at step %d", step.StepID)
		}
	}

	return nil
}

// VerifyTerminates verifies that the pipeline completes within the given timeout.
func VerifyTerminates(pipeline state.NewStep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		state.Run(ctx, pipeline)
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("pipeline did not terminate within %v", timeout)
	}
}

// VerifyProgress verifies that the pipeline makes progress and doesn't
// execute more than maxSteps steps (to detect infinite loops).
func VerifyProgress(pipeline state.NewStep, maxSteps int) error {
	tracer, trace := NewTracer()
	wrapped := state.WithMiddleware(pipeline, tracer)
	state.Run(context.Background(), wrapped)

	trace.mu.Lock()
	stepCount := len(trace.Steps)
	trace.mu.Unlock()

	if stepCount > maxSteps {
		return fmt.Errorf("exceeded max steps: %d > %d (possible infinite loop)", stepCount, maxSteps)
	}

	return nil
}
