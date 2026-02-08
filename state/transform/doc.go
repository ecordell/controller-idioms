// Package transform provides utilities for creating recursive middleware
// that propagates through continuation chains in state pipelines.
//
// The key insight is that middleware can wrap not just the current step,
// but also the next continuation and any returned steps, allowing
// transformations to "thread through" entire pipeline executions.
//
// # Basic Usage
//
// Create recursive middleware that logs before/after each step:
//
//	logger := transform.RecursiveMiddleware(
//		func(ctx context.Context) { log.Println("before step") },
//		func(ctx context.Context) { log.Println("after step") },
//	)
//
//	pipeline := state.Sequence(step1, step2, step3)
//	wrapped := state.WithMiddleware(pipeline, logger)
//	state.Run(ctx, wrapped)
//
// # Context-Based Transforms
//
// Register transforms in context for automatic application:
//
//	ctx = transform.WithTransform(ctx, loggingMiddleware)
//	ctx = transform.WithTransform(ctx, metricsMiddleware)
//
//	transform.RunWithTransforms(ctx, pipeline)
//
// # Writing Custom Recursive Middleware
//
// For more complex middleware, use the RecursiveMiddleware helper as a template
// and implement custom logic:
//
//	func CustomMiddleware(config Config) state.Middleware {
//		var wrap func(state.NewStep) state.NewStep
//		wrap = func(step state.NewStep) state.NewStep {
//			return func(next state.Step) state.Step {
//				wrappedNext := transform.WrapStep(next, wrap)
//				return state.StepFunc(func(ctx context.Context) state.Step {
//					// Before logic
//					result := step(wrappedNext).Run(ctx)
//					// After logic
//					if result != nil {
//						return transform.WrapStep(result, wrap)
//					}
//					return result
//				})
//			}
//		}
//		return wrap
//	}
package transform
