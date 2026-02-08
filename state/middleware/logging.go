package middleware

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// RecursiveLogging creates middleware that logs before and after each step execution.
// The logFunc receives a formatted log message for each step.
//
// Each step is assigned a unique ID for tracing through the pipeline.
func RecursiveLogging(logFunc func(string)) state.Middleware {
	var stepCounter atomic.Int64

	return transform.RecursiveMiddleware(
		func(ctx context.Context) {
			id := stepCounter.Add(1)
			logFunc(fmt.Sprintf("[step-%d] executing step", id))
		},
		func(ctx context.Context) {
			id := stepCounter.Load()
			logFunc(fmt.Sprintf("[step-%d] completed step", id))
		},
	)
}
