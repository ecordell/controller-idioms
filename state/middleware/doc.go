// Package middleware provides common recursive middleware implementations
// for cross-cutting concerns in state pipelines.
//
// All middleware in this package uses recursive wrapping to propagate
// through entire pipeline executions, including continuations and branches.
//
// # Available Middleware
//
// - RecursiveLogging: Log before/after each step
// - RecursiveMetrics: Record timing and execution counts
// - RecursiveCircuitBreaker: Stop execution after N failures
// - RecursiveRateLimit: Throttle execution between steps
//
// # Usage
//
//	import (
//		"github.com/authzed/controller-idioms/state"
//		"github.com/authzed/controller-idioms/state/middleware"
//		"github.com/authzed/controller-idioms/state/transform"
//	)
//
//	pipeline := state.Sequence(step1, step2, step3)
//
//	// Apply explicit middleware
//	logged := state.WithMiddleware(pipeline, middleware.RecursiveLogging(logger))
//	state.Run(ctx, logged)
//
//	// Or use context-based automatic application
//	ctx = transform.WithTransform(ctx, middleware.RecursiveLogging(logger))
//	ctx = transform.WithTransform(ctx, middleware.RecursiveMetrics(collector))
//	transform.RunWithTransforms(ctx, pipeline)
//
// # Composing Multiple Middleware
//
// Multiple middleware can be applied to the same pipeline. They compose
// correctly and execute in the order applied (right-to-left in WithMiddleware):
//
//	wrapped := state.WithMiddleware(
//		pipeline,
//		middleware.RecursiveLogging(logger),
//		middleware.RecursiveMetrics(collector),
//		middleware.RecursiveCircuitBreaker(5),
//	)
//
// In this example, CircuitBreaker is outermost, then Metrics, then Logging.
package middleware
