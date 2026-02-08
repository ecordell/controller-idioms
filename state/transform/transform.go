package transform

import (
	"context"

	"github.com/authzed/controller-idioms/state"
)

// WrapStep wraps a Step with a recursive middleware wrapper.
// This is an internal utility for implementing recursive middleware.
// If step is nil, returns nil.
func WrapStep(step state.Step, wrapper func(state.NewStep) state.NewStep) state.Step {
	if step == nil {
		return nil
	}
	// Convert Step to NewStep, apply wrapper, then convert back
	return wrapper(func(next state.Step) state.Step {
		return step
	})(nil)
}

// RecursiveMiddleware creates a middleware that propagates through the
// entire continuation chain by recursively wrapping continuations.
//
// The before function is called before each step executes.
// The after function is called after each step executes.
//
// This middleware will wrap not just the top-level step, but also:
// - The next continuation passed to each step
// - Any Step returned from execution
//
// This allows the middleware to "thread through" the entire pipeline.
func RecursiveMiddleware(
	before func(ctx context.Context),
	after func(ctx context.Context),
) state.Middleware {
	var wrap func(state.NewStep) state.NewStep
	wrap = func(step state.NewStep) state.NewStep {
		return func(next state.Step) state.Step {
			// Create a wrappedNext that applies middleware recursively
			// When an Action calls wrappedNext.Run(), it returns a continuation
			// that, when run, applies middleware to the next step
			wrappedNext := state.StepFunc(func(ctx context.Context) state.Step {
				// Return a continuation that will apply middleware when run
				return state.StepFunc(func(ctx context.Context) state.Step {
					if before != nil {
						before(ctx)
					}

					var result state.Step
					if next != nil {
						result = next.Run(ctx)
					}

					if after != nil {
						after(ctx)
					}

					return result
				})
			})

			// Execute current step with wrapped next
			return state.StepFunc(func(ctx context.Context) state.Step {
				if before != nil {
					before(ctx)
				}

				result := step(wrappedNext).Run(ctx)

				if after != nil {
					after(ctx)
				}

				// Process any continuation returned
				// Each continuation needs to be wrapped and run with middleware
				for result != nil {
					wrapped := WrapStep(result, wrap)
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
