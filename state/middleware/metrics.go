package middleware

import (
	"context"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveMetrics creates middleware that records metrics for each step execution.
//
// incrementFunc is called before each step (for counting executions).
// recordDurationFunc is called after each step with the execution duration.
func RecursiveMetrics(
	incrementFunc func(),
	recordDurationFunc func(time.Duration),
) state.Middleware {
	var wrap func(state.NewStep) state.NewStep
	wrap = func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			// Create a wrappedNext that applies middleware recursively
			wrappedNext := state.StepFunc(func(ctx context.Context) state.Step {
				// Return a continuation that will apply middleware when run
				return state.StepFunc(func(ctx context.Context) state.Step {
					if incrementFunc != nil {
						incrementFunc()
					}
					start := time.Now()

					var result state.Step
					if next != nil {
						// Wrap next before running it
						wrappedNext := transform.WrapStep(next, wrap)
						if wrappedNext != nil {
							result = wrappedNext.Run(ctx)
						}
					}

					if recordDurationFunc != nil {
						recordDurationFunc(time.Since(start))
					}

					return result
				})
			})

			// Execute current step with wrapped next
			return state.StepFunc(func(ctx context.Context) state.Step {
				if incrementFunc != nil {
					incrementFunc()
				}
				start := time.Now()

				result := step(wrappedNext).Run(ctx)

				if recordDurationFunc != nil {
					recordDurationFunc(time.Since(start))
				}

				// Process any continuation returned
				for result != nil {
					wrapped := transform.WrapStep(result, wrap)
					if wrapped == nil {
						break
					}
					result = wrapped.Run(ctx)
				}

				return nil
			})
		}
	}
	return wrap
}
