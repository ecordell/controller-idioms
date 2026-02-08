package middleware

import (
	"context"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveRateLimit creates middleware that rate limits step execution.
//
// The waitFunc is called before each step execution and should block
// until the step is allowed to proceed. It should respect context
// cancellation.
//
// Example waitFunc implementations:
// - time.Sleep wrapper
// - golang.org/x/time/rate.Limiter.Wait
// - Custom token bucket implementation
func RecursiveRateLimit(waitFunc func(context.Context) error) state.Middleware {
	return transform.RecursiveMiddleware(
		func(ctx context.Context) {
			if waitFunc != nil {
				// Ignore error - if context is canceled, the step will handle it
				_ = waitFunc(ctx)
			}
		},
		nil, // No after logic needed
	)
}
