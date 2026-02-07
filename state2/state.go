// Package state2 provides a continuation-passing style framework for building
// composable state machines and processing pipelines in Kubernetes controllers.
//
// This is a clean implementation focusing on the core abstractions needed
// for controller logic composition.
package state2

import (
	"context"
)

// Handler represents a single step in a computation that returns the next handler
// to execute, or nil to terminate the pipeline.
//
// The continuation-passing style eliminates the need for complex builder patterns
// and ID-based handler lookup, making composition more direct and intuitive.
type Handler interface {
	Handle(context.Context) Handler
}

// HandlerFunc is a function type that implements Handler.
type HandlerFunc func(ctx context.Context) Handler

func (f HandlerFunc) Handle(ctx context.Context) Handler {
	return f(ctx)
}

// NewHandler is a factory function that creates a Handler when given the next
// handler to execute. This is the basic building block for composing handler
// pipelines using continuation-passing style.
//
// The next parameter represents what should happen after this handler completes.
// If next is nil, the pipeline terminates after this handler.
type NewHandler func(next Handler) Handler

// Handler converts a NewHandler to a Handler by calling it with nil as the next handler.
// This is useful for executing a NewHandler as the final step in a pipeline.
func (nh NewHandler) Handler() Handler {
	return nh(nil)
}

// Step creates a NewHandler that executes the given function and then continues
// to the next handler in the pipeline.
//
// This is the most basic building block for creating handler steps that perform
// side effects (like updating Kubernetes resources, logging, etc.) and then
// continue processing.
//
// Usage:
//   validateInput := state2.Step(func(ctx context.Context) {
//       // validation logic here
//       if invalid {
//           // set error in context or take other action
//       }
//   })
func Step(fn func(context.Context)) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the step function
			fn(ctx)

			// Continue to next handler if present
			if next != nil {
				return next.Handle(ctx)
			}

			// Terminate pipeline
			return nil
		})
	}
}
