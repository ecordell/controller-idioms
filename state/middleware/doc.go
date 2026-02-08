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
// - RecursiveTracing: OpenTelemetry distributed tracing
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
package middleware
