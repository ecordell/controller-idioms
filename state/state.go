// Package state contains Step units used to compose reconciliation loops
// for controllers.
//
// You write functions or types that implement the Handler interface, and
// then compose them via NewStep functions and composition operators.
//
// NewStep functions are used for creating Handler instances, and Handlers
// are the things that actually process requests - each handler returns
// the next handler to execute, or nil to terminate.
//
// Writing controllers in this style permits composition patterns including Sequence,
// Parallel, Decision, and other operations like Map and Bind.
package state

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Handler represents a handler in a processing pipeline.
// Each handler processes a context and returns the next handler to execute, or nil to terminate.
type Handler interface {
	Handle(context.Context) Handler
}

// HandlerFunc is a function type that implements Handler
type HandlerFunc func(ctx context.Context) Handler

func (f HandlerFunc) Handle(ctx context.Context) Handler {
	return f(ctx)
}

// NewHandler is a function that creates a Handler when given the next handler to execute.
// This is the basic building block for composing handler pipelines using continuation-passing style.
type NewHandler func(next Handler) Handler

// Handler converts a NewHandler to a Handler by calling it with nil as the next handler.
func (nh NewHandler) Handler() Handler {
	return nh(nil)
}

// Run executes a NewHandler pipeline until completion.
// With continuation-passing style, the pipeline executes completely in one call.
func Run(ctx context.Context, newHandler NewHandler) {
	handler := newHandler.Handler()
	if handler != nil {
		handler.Handle(ctx)
	}
}

// Terminal creates a handler that terminates the pipeline.
var Terminal = NewHandler(func(next Handler) Handler {
	return HandlerFunc(func(ctx context.Context) Handler {
		return nil
	})
})

// Noop creates a handler that does nothing and continues to the next handler.
var Noop = NewHandler(func(next Handler) Handler {
	return HandlerFunc(func(ctx context.Context) Handler {
		if next != nil {
			return next.Handle(ctx)
		}
		return nil
	})
})

// Action creates a handler that performs an action and then continues to the next handler.
// Note: Action cannot modify context. Use Step() for context transformations.
func Action(action func(context.Context)) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			action(ctx)
			if next != nil {
				return next.Handle(ctx)
			}
			return nil
		})
	}
}

// Step lifts a pure context transformation into the Handler monad.
// This is the formal "return" or "unit" operation for our Handler monad.
// Mathematical signature: (Context -> Context) -> NewHandler
func Step(transform func(context.Context) context.Context) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			newCtx := transform(ctx)
			if next != nil {
				return next.Handle(newCtx)
			}
			return nil
		})
	}
}

// Then creates a handler that performs an action and continues to the specified handler.
func Then(action func(context.Context), nextHandler NewHandler) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			action(ctx)
			return nextHandler.Handler().Handle(ctx)
		})
	}
}

// Sequence composes multiple NewHandler functions into a sequential pipeline.
// Each handler in the sequence executes in order with proper context threading.
func Sequence(handlers ...NewHandler) NewHandler {
	return func(next Handler) Handler {
		if len(handlers) == 0 {
			if next != nil {
				return next
			}
			return nil
		}

		// Build the chain right-to-left (continuation-passing style)
		current := next
		for i := len(handlers) - 1; i >= 0; i-- {
			current = handlers[i](current)
		}
		return current
	}
}

// Parallel composes multiple NewHandler functions to run in parallel,
// then continues to the next handler after all complete.
func Parallel(handlers ...NewHandler) NewHandler {
	return func(next Handler) Handler {
		handlerNames := make([]string, len(handlers))
		for i := range handlers {
			handlerNames[i] = fmt.Sprintf("handler_%d", i)
		}

		return &ParallelStep{
			handlers: handlers,
			next:     next,
			id:       fmt.Sprintf("parallel[%s]", strings.Join(handlerNames, ",")),
		}
	}
}

// ParallelStep implements parallel execution of handlers.
type ParallelStep struct {
	handlers []NewHandler
	next     Handler
	id       string
}

func (p *ParallelStep) Handle(ctx context.Context) Handler {
	var wg sync.WaitGroup

	for _, handler := range p.handlers {
		handler := handler
		wg.Add(1)
		go func() {
			defer wg.Done()
			handlerInstance := handler.Handler()
			if handlerInstance != nil {
				handlerInstance.Handle(ctx)
			}
		}()
	}

	wg.Wait()

	if p.next != nil {
		return p.next.Handle(ctx)
	}
	return nil
}

// Decision creates a conditional handler that chooses between two paths.
func Decision(predicate func(context.Context) bool, trueHandler, falseHandler NewHandler) NewHandler {
	return func(next Handler) Handler {
		return &DecisionStep{
			predicate:    predicate,
			trueHandler:  trueHandler,
			falseHandler: falseHandler,
			next:         next,
		}
	}
}

// DecisionStep implements conditional execution.
type DecisionStep struct {
	predicate    func(context.Context) bool
	trueHandler  NewHandler
	falseHandler NewHandler
	next         Handler
}

func (d *DecisionStep) Handle(ctx context.Context) Handler {
	var chosen NewHandler
	if d.predicate(ctx) {
		chosen = d.trueHandler
	} else {
		chosen = d.falseHandler
	}

	return chosen(d.next).Handle(ctx)
}

// Enum creates a multi-way branching handler based on a selector function.
// The selector function returns a value that is matched against the cases map.
// If no match is found, the defaultHandler is executed.
func Enum[T comparable](
	selector func(context.Context) T,
	cases map[T]NewHandler,
	defaultHandler NewHandler,
) NewHandler {
	return func(next Handler) Handler {
		return &EnumStep[T]{
			selector:       selector,
			cases:          cases,
			defaultHandler: defaultHandler,
			next:           next,
		}
	}
}

// EnumStep implements multi-way branching based on enum values.
type EnumStep[T comparable] struct {
	selector       func(context.Context) T
	cases          map[T]NewHandler
	defaultHandler NewHandler
	next           Handler
}

func (e *EnumStep[T]) Handle(ctx context.Context) Handler {
	value := e.selector(ctx)
	var chosen NewHandler
	if handler, ok := e.cases[value]; ok {
		chosen = handler
	} else if e.defaultHandler != nil {
		chosen = e.defaultHandler
	} else {
		// No match and no default, continue to next
		if e.next != nil {
			return e.next.Handle(ctx)
		}
		return nil
	}

	return chosen(e.next).Handle(ctx)
}

// Switch creates a multi-way branching handler based on string values.
// This is a convenience function for the common case of string-based branching.
func Switch(
	selector func(context.Context) string,
	cases map[string]NewHandler,
	defaultHandler NewHandler,
) NewHandler {
	return Enum(selector, cases, defaultHandler)
}

// Map transforms the context before passing it to the next handler.
// This is the monadic map operation.
func Map(transform func(context.Context) context.Context, handler NewHandler) NewHandler {
	return func(next Handler) Handler {
		return &MapStep{
			transform: transform,
			handler:   handler,
			next:      next,
		}
	}
}

// MapStep implements context transformation.
type MapStep struct {
	transform func(context.Context) context.Context
	handler   NewHandler
	next      Handler
}

func (m *MapStep) Handle(ctx context.Context) Handler {
	transformedCtx := m.transform(ctx)
	return m.handler(m.next).Handle(transformedCtx)
}

// Bind is the monadic bind operation for handlers.
// It allows chaining handlers where the next handler depends on the result of the current one.
func Bind(handler NewHandler, f func() NewHandler) NewHandler {
	return func(next Handler) Handler {
		return &BindStep{
			handler: handler,
			f:       f,
			next:    next,
		}
	}
}

// BindStep implements monadic bind.
type BindStep struct {
	handler NewHandler
	f       func() NewHandler
	next    Handler
}

func (b *BindStep) Handle(ctx context.Context) Handler {
	// Create a continuation that first runs the bound function, then continues to next
	continuation := Sequence(b.f())(b.next)
	return b.handler(continuation).Handle(ctx)
}

// Return lifts a simple action into the Handler monad.
// This is the monadic return/unit operation.
func Return(action func(context.Context)) NewHandler {
	return Action(action)
}

// Choice provides an escape mechanism similar to callCC.
// The provided function receives escape functions that can jump to different continuations.
func Choice(f func(escapes EscapeOptions) NewHandler) NewHandler {
	return func(next Handler) Handler {
		return &ChoiceStep{f: f, next: next}
	}
}

// EscapeOptions provides escape continuations for Choice.
type EscapeOptions struct {
	// Terminate immediately
	Done func() NewHandler
	// Continue with a specific handler
	Continue func(NewHandler) NewHandler
}

// ChoiceStep implements the choice/callCC operation.
type ChoiceStep struct {
	f       func(EscapeOptions) NewHandler
	escaped bool
	next    Handler
}

func (c *ChoiceStep) Handle(ctx context.Context) Handler {
	escapes := EscapeOptions{
		Done: func() NewHandler {
			return func(next Handler) Handler {
				return nil
			}
		},
		Continue: func(handler NewHandler) NewHandler {
			return handler
		},
	}

	result := c.f(escapes)
	return result(c.next).Handle(ctx)
}

// Middleware represents a function that wraps a handler with additional behavior.
// It takes a handler and returns a new handler that includes the middleware behavior.
type Middleware func(NewHandler) NewHandler

// WithMiddleware applies middleware to a handler, creating a new handler that
// executes the middleware behavior around the original handler.
func WithMiddleware(handler NewHandler, middlewares ...Middleware) NewHandler {
	result := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	return result
}

// ConditionalMiddleware creates middleware that only executes the wrapped handler
// if the condition is true, otherwise it skips to the next handler.
func ConditionalMiddleware(condition func(context.Context) bool) Middleware {
	return func(handler NewHandler) NewHandler {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				if !condition(ctx) {
					if next != nil {
						return next.Handle(ctx)
					}
					return nil
				}
				return handler(next).Handle(ctx)
			})
		}
	}
}

// ErrorHandlingMiddleware creates middleware that handles errors and cancellation.
func ErrorHandlingMiddleware(onError func(context.Context, error)) Middleware {
	return func(handler NewHandler) NewHandler {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				// Execute the wrapped handler
				wrappedHandler := handler(next)
				if wrappedHandler == nil {
					if next != nil {
						return next.Handle(ctx)
					}
					return nil
				}

				result := wrappedHandler.Handle(ctx)

				// Check for errors after execution
				if err := ctx.Err(); err != nil {
					if onError != nil {
						onError(ctx, err)
					}
					return nil
				}

				return result
			})
		}
	}
}

// LoggingMiddleware creates middleware that logs before and after handler execution.
func LoggingMiddleware(handlerName string, logger func(string)) Middleware {
	return func(handler NewHandler) NewHandler {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				if logger != nil {
					logger("entering " + handlerName)
				}

				wrappedHandler := handler(next)
				if wrappedHandler == nil {
					if logger != nil {
						logger("exiting " + handlerName)
					}
					if next != nil {
						return next.Handle(ctx)
					}
					return nil
				}

				result := wrappedHandler.Handle(ctx)

				if logger != nil {
					logger("exiting " + handlerName)
				}

				return result
			})
		}
	}
}

// ValidationMiddleware creates middleware that performs validation before
// executing the wrapped handler. If validation fails, it skips the handler.
func ValidationMiddleware(validate func(context.Context) bool, onValidationFailure func(context.Context)) Middleware {
	return func(handler NewHandler) NewHandler {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				if !validate(ctx) {
					if onValidationFailure != nil {
						onValidationFailure(ctx)
					}
					if next != nil {
						return next.Handle(ctx)
					}
					return nil
				}
				return handler(next).Handle(ctx)
			})
		}
	}
}

// WrapWithWork creates middleware that executes work before the wrapped handler.
func WrapWithWork(work func(context.Context)) Middleware {
	return func(handler NewHandler) NewHandler {
		return func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				work(ctx)

				// Check for cancellation after work
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}

				return handler(next).Handle(ctx)
			})
		}
	}
}

// CallAndCheck executes a handler and then allows inspection of the result
// before deciding what to do next.
func CallAndCheck(
	toCall NewHandler,
	onResult func(ctx context.Context, result Handler) Handler,
) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the called handler
			handler := toCall(nil)
			if handler == nil {
				resultHandler := onResult(ctx, nil)
				if resultHandler != nil {
					return resultHandler.Handle(ctx)
				}
				if next != nil {
					return next.Handle(ctx)
				}
				return nil
			}

			result := handler.Handle(ctx)
			resultHandler := onResult(ctx, result)
			if resultHandler != nil {
				return resultHandler.Handle(ctx)
			}
			if next != nil {
				return next.Handle(ctx)
			}
			return nil
		})
	}
}

// CallAndContinueIf executes a handler and continues to the next handler only if
// a condition is met after execution.
func CallAndContinueIf(
	toCall NewHandler,
	condition func(context.Context) bool,
	onTrue NewHandler,
	onFalse NewHandler,
) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the called handler
			handler := toCall(nil)
			if handler != nil {
				handler.Handle(ctx)
			}

			// Check condition and branch
			if condition(ctx) {
				if onTrue != nil {
					return onTrue(next).Handle(ctx)
				}
				if next != nil {
					return next.Handle(ctx)
				}
				return nil
			}
			if onFalse != nil {
				return onFalse(next).Handle(ctx)
			}
			return nil // Stop execution
		})
	}
}

// CallAndContinue executes a handler and then unconditionally continues to
// another handler, regardless of the first handler's result.
func CallAndContinue(toCall NewHandler, then NewHandler) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the called handler to completion
			handler := toCall.Handler()
			if handler != nil {
				handler.Handle(ctx)
			}

			// Then continue to the specified handler
			if then != nil {
				return then(next).Handle(ctx)
			}
			if next != nil {
				return next.Handle(ctx)
			}
			return nil
		})
	}
}

// CallAndStop executes a handler and then stops, ignoring any continuation
// the called handler might have returned.
func CallAndStop(toCall NewHandler) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the called handler to completion
			handler := toCall.Handler()
			if handler != nil {
				handler.Handle(ctx)
			}
			return nil // Always stop after execution
		})
	}
}

// Reuse allows a handler to be reused by calling it and then deciding what
// to do based on context state after execution.
func Reuse(
	handlerToReuse NewHandler,
	afterExecution func(context.Context) NewHandler,
) NewHandler {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context) Handler {
			// Execute the reused handler
			handler := handlerToReuse.Handler()
			if handler != nil {
				handler.Handle(ctx)
			}

			// Decide what to do next based on context state
			nextHandler := afterExecution(ctx)
			if nextHandler != nil {
				return nextHandler(next).Handle(ctx)
			}
			if next != nil {
				return next.Handle(ctx)
			}
			return nil
		})
	}
}
