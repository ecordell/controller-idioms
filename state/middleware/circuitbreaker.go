package middleware

import (
	"context"
	"sync"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveCircuitBreaker creates middleware that stops execution after
// maxFailures error occurrences.
//
// An error is counted when the context has an error (ctx.Err() != nil).
// Once the circuit is open, all subsequent steps are skipped.
func RecursiveCircuitBreaker(maxFailures int) state.Middleware {
	var (
		failures int
		mu       sync.Mutex
	)

	var wrap func(state.NewStep) state.NewStep
	wrap = func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			wrappedNext := transform.WrapStep(next, wrap)

			return state.StepFunc(func(ctx context.Context) state.Step {
				// Check if circuit is open
				mu.Lock()
				isOpen := failures >= maxFailures
				mu.Unlock()

				if isOpen {
					// Circuit open - skip execution
					return nil
				}

				// Execute step
				result := step(wrappedNext).Run(ctx)

				// Check for error after execution
				if ctx.Err() != nil {
					mu.Lock()
					failures++
					mu.Unlock()
				}

				// Wrap result
				if result != nil {
					return transform.WrapStep(result, wrap)
				}
				return result
			})
		}
	}
	return wrap
}
