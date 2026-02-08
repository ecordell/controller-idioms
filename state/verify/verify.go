package verify

import (
	"context"
	"fmt"

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
