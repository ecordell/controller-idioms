// Package queue provides helpers for working with client-go's `workqueues` and control flow handlers.
//
// This file provides handlers that integrate with the state package to provide clean
// control flow patterns for controllers. Instead of manually calling queue operations
// and returning, controllers can now use:
//
//   return queue.Done()
//   return queue.Requeue()
//   return queue.RequeueAfter(5 * time.Second)
//   return queue.RequeueErr(err)
//
// These handlers automatically call the appropriate queue operations and terminate
// the handler pipeline by returning nil.
package queue

import (
	"context"
	"time"

	"github.com/authzed/controller-idioms/state"
)

// Done creates a handler that marks the current queue key as finished and terminates the pipeline.
// This is equivalent to calling queue.NewQueueOperationsCtx().Done(ctx) and returning.
//
// Usage:
//   pipeline := state.Sequence(
//     validateInput,
//     processResource,
//     queue.Done(), // Stop here - processing complete
//   )
func Done() state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			NewQueueOperationsCtx().Done(ctx)
			return nil // Terminate pipeline
		})
	}
}

// Requeue creates a handler that requeues the current key immediately and terminates the pipeline.
// This is equivalent to calling queue.NewQueueOperationsCtx().Requeue(ctx) and returning.
//
// Usage:
//   pipeline := state.Decision(
//     resourceReady,
//     continueProcessing,
//     queue.Requeue(), // Not ready - try again immediately
//   )
func Requeue() state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			NewQueueOperationsCtx().Requeue(ctx)
			return nil // Terminate pipeline
		})
	}
}

// RequeueAfter creates a handler that requeues the current key after the specified duration
// and terminates the pipeline.
// This is equivalent to calling queue.NewQueueOperationsCtx().RequeueAfter(ctx, duration) and returning.
//
// Usage:
//   pipeline := state.Decision(
//     resourceReady,
//     continueProcessing,
//     queue.RequeueAfter(30 * time.Second), // Not ready - try again in 30s
//   )
func RequeueAfter(duration time.Duration) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			NewQueueOperationsCtx().RequeueAfter(ctx, duration)
			return nil // Terminate pipeline
		})
	}
}

// RequeueErr creates a handler that records an error and requeues the current key immediately,
// then terminates the pipeline.
// This is equivalent to calling queue.NewQueueOperationsCtx().RequeueErr(ctx, err) and returning.
//
// Usage:
//   pipeline := state.Sequence(
//     validateInput,
//     state.Decision(
//       inputValid,
//       continueProcessing,
//       queue.RequeueErr(fmt.Errorf("validation failed")), // Invalid - requeue with error
//     ),
//   )
func RequeueErr(err error) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			NewQueueOperationsCtx().RequeueErr(ctx, err)
			return nil // Terminate pipeline
		})
	}
}

// RequeueAPIErr creates a handler that handles API errors with appropriate retry logic
// and terminates the pipeline.
// This checks if the error contains retry information from the API server and requeues
// accordingly, equivalent to calling queue.NewQueueOperationsCtx().RequeueAPIErr(ctx, err).
//
// Usage:
//   pipeline := state.Sequence(
//     callKubernetesAPI,
//     state.Decision(
//       apiCallSucceeded,
//       continueProcessing,
//       queue.RequeueAPIErr(apiError), // Handle API error with proper retry logic
//     ),
//   )
func RequeueAPIErr(err error) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			NewQueueOperationsCtx().RequeueAPIErr(ctx, err)
			return nil // Terminate pipeline
		})
	}
}

// ConditionalRequeue creates a handler that requeues based on a condition.
// If the condition is true, it requeues immediately. Otherwise, it continues to the next handler.
//
// Usage:
//   pipeline := state.Sequence(
//     checkResourceState,
//     queue.ConditionalRequeue(func(ctx context.Context) bool {
//       return !resourceIsReady(ctx)
//     }),
//     continueProcessing, // Only reached if resource is ready
//   )
func ConditionalRequeue(condition func(context.Context) bool) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			if condition(ctx) {
				NewQueueOperationsCtx().Requeue(ctx)
				return nil // Terminate pipeline
			}
			// Continue to next handler
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// ConditionalRequeueAfter creates a handler that requeues after a duration based on a condition.
// If the condition is true, it requeues after the specified duration. Otherwise, it continues to the next handler.
//
// Usage:
//   pipeline := state.Sequence(
//     checkResourceState,
//     queue.ConditionalRequeueAfter(
//       func(ctx context.Context) bool { return !resourceIsReady(ctx) },
//       5 * time.Minute,
//     ),
//     continueProcessing, // Only reached if resource is ready
//   )
func ConditionalRequeueAfter(condition func(context.Context) bool, duration time.Duration) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			if condition(ctx) {
				NewQueueOperationsCtx().RequeueAfter(ctx, duration)
				return nil // Terminate pipeline
			}
			// Continue to next handler
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// ConditionalDone creates a handler that marks processing as done based on a condition.
// If the condition is true, it calls Done(). Otherwise, it continues to the next handler.
//
// Usage:
//   pipeline := state.Sequence(
//     processResource,
//     queue.ConditionalDone(func(ctx context.Context) bool {
//       return processingComplete(ctx)
//     }),
//     handleIncompleteProcessing, // Only reached if processing not complete
//   )
func ConditionalDone(condition func(context.Context) bool) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			if condition(ctx) {
				NewQueueOperationsCtx().Done(ctx)
				return nil // Terminate pipeline
			}
			// Continue to next handler
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// OnError creates a handler that executes different queue operations based on whether
// an error occurred in the context.
//
// Usage:
//   pipeline := state.Sequence(
//     riskyOperation,
//     queue.OnError(
//       queue.RequeueErr(fmt.Errorf("operation failed")), // If error
//       queue.Done(), // If success
//     ),
//   )
func OnError(errorHandler, successHandler state.NewStep) state.NewStep {
	return func(next state.Step) state.Step {
		return state.StepFunc(func(ctx context.Context) state.Step {
			if ctx.Err() != nil {
				return errorHandler.Step().Run(ctx)
			}
			return successHandler.Step().Run(ctx)
		})
	}
}
