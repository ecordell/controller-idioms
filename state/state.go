// Package state contains Step units used to compose reconciliation loops
// for controllers.
//
// You write functions or types that implement the Step interface, and
// then compose them via NewStep functions and composition operators.
//
// NewStep functions are used for creating Step instances, and Handlers
// are the things that actually process requests - each step returns
// the next step to execute, or nil to terminate.
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

// Step represents a step in a processing pipeline.
// Each step processes a context and returns the next step to execute, or nil to terminate.
type Step interface {
	Run(context.Context) Step
}

// StepFunc is a function type that implements Step
type StepFunc func(ctx context.Context) Step

func (f StepFunc) Run(ctx context.Context) Step {
	return f(ctx)
}

// NewStep is a function that creates a Step when given the next step to execute.
// This is the basic building block for composing step pipelines using continuation-passing style.
type NewStep func(next Step) Step

// Step converts a NewStep to a Step by calling it with nil as the next step.
func (ns NewStep) Step() Step {
	return ns(nil)
}

// Run executes a NewStep pipeline until completion.
// With continuation-passing style, the pipeline executes completely in one call.
func Run(ctx context.Context, newStep NewStep) {
	step := newStep.Step()
	if step != nil {
		step.Run(ctx)
	}
}

// Terminal creates a step that terminates the pipeline.
var Terminal = NewStep(func(next Step) Step {
	return StepFunc(func(ctx context.Context) Step {
		return nil
	})
})

// Noop creates a step that does nothing and continues to the next step.
var Noop = NewStep(func(next Step) Step {
	return StepFunc(func(ctx context.Context) Step {
		if next != nil {
			return next.Run(ctx)
		}
		return nil
	})
})

// Action creates a step that performs an action and then continues to the next step.
// Note: Action cannot modify context. Use Step() for context transformations.
func Action(action func(context.Context)) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			action(ctx)
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// Do lifts a pure context transformation into the Step monad.
// This is the formal "return" or "unit" operation for our Step monad.
// Mathematical signature: (Context -> Context) -> NewStep
func Do(transform func(context.Context) context.Context) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			newCtx := transform(ctx)
			if next != nil {
				return next.Run(newCtx)
			}
			return nil
		})
	}
}

// Then creates a step that performs an action and continues to the specified step.
func Then(action func(context.Context), nextStep NewStep) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			action(ctx)
			return nextStep.Step().Run(ctx)
		})
	}
}

// Sequence composes multiple NewStep functions into a sequential pipeline.
// Each step in the sequence executes in order with proper context threading.
func Sequence(steps ...NewStep) NewStep {
	return func(next Step) Step {
		if len(steps) == 0 {
			if next != nil {
				return next
			}
			return nil
		}

		// Build the chain right-to-left (continuation-passing style)
		current := next
		for i := len(steps) - 1; i >= 0; i-- {
			current = steps[i](current)
		}
		return current
	}
}

// Parallel composes multiple NewStep functions to run in parallel,
// then continues to the next step after all complete.
func Parallel(steps ...NewStep) NewStep {
	return func(next Step) Step {
		stepNames := make([]string, len(steps))
		for i := range steps {
			stepNames[i] = fmt.Sprintf("handler_%d", i)
		}

		return &ParallelStep{
			steps: steps,
			next:     next,
			id:       fmt.Sprintf("parallel[%s]", strings.Join(stepNames, ",")),
		}
	}
}

// ParallelStep implements parallel execution of steps.
type ParallelStep struct {
	steps []NewStep
	next     Step
	id       string
}

func (p *ParallelStep) Run(ctx context.Context) Step {
	var wg sync.WaitGroup
	var panicMu sync.Mutex
	var panicErr error

	for _, step := range p.steps {
		step := step
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicMu.Lock()
					defer panicMu.Unlock()
					if panicErr == nil {
						panicErr = fmt.Errorf("panic in parallel step: %v", r)
					}
				}
			}()

			// Check for cancellation before executing
			select {
			case <-ctx.Done():
				return
			default:
			}

			stepInstance := step.Step()
			if stepInstance != nil {
				stepInstance.Run(ctx)
			}
		}()
	}

	wg.Wait()

	// If any goroutine panicked, we should not continue
	if panicErr != nil {
		// In a real implementation, you might want to log this or handle it differently
		// For now, we just stop the pipeline
		return nil
	}

	// Check for cancellation after parallel execution
	if ctx.Err() != nil {
		return nil
	}

	if p.next != nil {
		return p.next.Run(ctx)
	}
	return nil
}

// Decision creates a conditional step that chooses between two paths.
func Decision(predicate func(context.Context) bool, trueHandler, falseHandler NewStep) NewStep {
	return func(next Step) Step {
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
	trueHandler  NewStep
	falseHandler NewStep
	next         Step
}

func (d *DecisionStep) Run(ctx context.Context) Step {
	var chosen NewStep
	if d.predicate(ctx) {
		chosen = d.trueHandler
	} else {
		chosen = d.falseHandler
	}

	return chosen(d.next).Run(ctx)
}

// Enum creates a multi-way branching step based on a selector function.
// The selector function returns a value that is matched against the cases map.
// If no match is found, the defaultHandler is executed.
func Enum[T comparable](
	selector func(context.Context) T,
	cases map[T]NewStep,
	defaultHandler NewStep,
) NewStep {
	return func(next Step) Step {
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
	cases          map[T]NewStep
	defaultHandler NewStep
	next           Step
}

func (e *EnumStep[T]) Run(ctx context.Context) Step {
	value := e.selector(ctx)
	var chosen NewStep
	if step, ok := e.cases[value]; ok {
		chosen = step
	} else if e.defaultHandler != nil {
		chosen = e.defaultHandler
	} else {
		// No match and no default, continue to next
		if e.next != nil {
			return e.next.Run(ctx)
		}
		return nil
	}

	return chosen(e.next).Run(ctx)
}

// Switch creates a multi-way branching step based on string values.
// This is a convenience function for the common case of string-based branching.
func Switch(
	selector func(context.Context) string,
	cases map[string]NewStep,
	defaultHandler NewStep,
) NewStep {
	return Enum(selector, cases, defaultHandler)
}

// Map transforms the context before passing it to the next step.
// This is the monadic map operation.
func Map(transform func(context.Context) context.Context, step NewStep) NewStep {
	return func(next Step) Step {
		return &MapStep{
			transform: transform,
			step:   step,
			next:      next,
		}
	}
}

// MapStep implements context transformation.
type MapStep struct {
	transform func(context.Context) context.Context
	step   NewStep
	next      Step
}

func (m *MapStep) Run(ctx context.Context) Step {
	transformedCtx := m.transform(ctx)
	return m.step(m.next).Run(transformedCtx)
}

// Bind is the monadic bind operation for steps.
// It allows chaining steps where the next step depends on the result of the current one.
func Bind(step NewStep, f func() NewStep) NewStep {
	return func(next Step) Step {
		return &BindStep{
			step: step,
			f:       f,
			next:    next,
		}
	}
}

// BindStep implements monadic bind.
type BindStep struct {
	step NewStep
	f       func() NewStep
	next    Step
}

func (b *BindStep) Run(ctx context.Context) Step {
	// Create a continuation that first runs the bound function, then continues to next
	continuation := Sequence(b.f())(b.next)
	return b.step(continuation).Run(ctx)
}

// Return lifts a simple action into the Step monad.
// This is the monadic return/unit operation.
func Return(action func(context.Context)) NewStep {
	return Action(action)
}

// Choice provides an escape mechanism similar to callCC.
// The provided function receives escape functions that can jump to different continuations.
func Choice(f func(escapes EscapeOptions) NewStep) NewStep {
	return func(next Step) Step {
		return &ChoiceStep{f: f, next: next}
	}
}

// EscapeOptions provides escape continuations for Choice.
type EscapeOptions struct {
	// Terminate immediately
	Done func() NewStep
	// Continue with a specific step
	Continue func(NewStep) NewStep
}

// ChoiceStep implements the choice/callCC operation.
type ChoiceStep struct {
	f       func(EscapeOptions) NewStep
	escaped bool
	next    Step
}

func (c *ChoiceStep) Run(ctx context.Context) Step {
	escapes := EscapeOptions{
		Done: func() NewStep {
			return func(next Step) Step {
				return nil
			}
		},
		Continue: func(step NewStep) NewStep {
			return step
		},
	}

	result := c.f(escapes)
	return result(c.next).Run(ctx)
}

// Middleware represents a function that wraps a step with additional behavior.
// It takes a step and returns a new step that includes the middleware behavior.
type Middleware func(NewStep) NewStep

// WithMiddleware applies middleware to a step, creating a new step that
// executes the middleware behavior around the original step.
func WithMiddleware(step NewStep, middlewares ...Middleware) NewStep {
	result := step
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	return result
}

// ConditionalMiddleware creates middleware that only executes the wrapped step
// if the condition is true, otherwise it skips to the next step.
func ConditionalMiddleware(condition func(context.Context) bool) Middleware {
	return func(step NewStep) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				if !condition(ctx) {
					if next != nil {
						return next.Run(ctx)
					}
					return nil
				}
				return step(next).Run(ctx)
			})
		}
	}
}

// ErrorHandlingMiddleware creates middleware that handles errors and cancellation.
func ErrorHandlingMiddleware(onError func(context.Context, error)) Middleware {
	return func(step NewStep) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				// Execute the wrapped step
				wrappedStep := step(next)
				if wrappedStep == nil {
					if next != nil {
						return next.Run(ctx)
					}
					return nil
				}

				result := wrappedStep.Run(ctx)

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

// LoggingMiddleware creates middleware that logs before and after step execution.
func LoggingMiddleware(stepName string, logger func(string)) Middleware {
	return func(step NewStep) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				if logger != nil {
					logger("entering " + stepName)
				}

				wrappedStep := step(next)
				if wrappedStep == nil {
					if logger != nil {
						logger("exiting " + stepName)
					}
					if next != nil {
						return next.Run(ctx)
					}
					return nil
				}

				result := wrappedStep.Run(ctx)

				if logger != nil {
					logger("exiting " + stepName)
				}

				return result
			})
		}
	}
}

// ValidationMiddleware creates middleware that performs validation before
// executing the wrapped step. If validation fails, it skips the step.
func ValidationMiddleware(validate func(context.Context) bool, onValidationFailure func(context.Context)) Middleware {
	return func(step NewStep) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				if !validate(ctx) {
					if onValidationFailure != nil {
						onValidationFailure(ctx)
					}
					if next != nil {
						return next.Run(ctx)
					}
					return nil
				}
				return step(next).Run(ctx)
			})
		}
	}
}

// WrapWithWork creates middleware that executes work before the wrapped step.
func WrapWithWork(work func(context.Context)) Middleware {
	return func(step NewStep) NewStep {
		return func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				work(ctx)

				// Check for cancellation after work
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}

				return step(next).Run(ctx)
			})
		}
	}
}

// CallAndCheck executes a step and then allows inspection of the result
// before deciding what to do next.
func CallAndCheck(
	toCall NewStep,
	onResult func(ctx context.Context, result Step) Step,
) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute the called step
			step := toCall(nil)
			if step == nil {
				resultHandler := onResult(ctx, nil)
				if resultHandler != nil {
					return resultHandler.Run(ctx)
				}
				if next != nil {
					return next.Run(ctx)
				}
				return nil
			}

			result := step.Run(ctx)
			resultHandler := onResult(ctx, result)
			if resultHandler != nil {
				return resultHandler.Run(ctx)
			}
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// CallAndContinueIf executes a step and continues to the next step only if
// a condition is met after execution.
func CallAndContinueIf(
	toCall NewStep,
	condition func(context.Context) bool,
	onTrue NewStep,
	onFalse NewStep,
) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute the called step
			step := toCall(nil)
			if step != nil {
				step.Run(ctx)
			}

			// Check condition and branch
			if condition(ctx) {
				if onTrue != nil {
					return onTrue(next).Run(ctx)
				}
				if next != nil {
					return next.Run(ctx)
				}
				return nil
			}
			if onFalse != nil {
				return onFalse(next).Run(ctx)
			}
			return nil // Stop execution
		})
	}
}

// CallAndContinue executes a step and then unconditionally continues to
// another step, regardless of the first step's result.
func CallAndContinue(toCall NewStep, then NewStep) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute the called step to completion
			step := toCall.Step()
			if step != nil {
				step.Run(ctx)
			}

			// Then continue to the specified step
			if then != nil {
				return then(next).Run(ctx)
			}
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}

// CallAndStop executes a step and then stops, ignoring any continuation
// the called step might have returned.
func CallAndStop(toCall NewStep) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute the called step to completion
			step := toCall.Step()
			if step != nil {
				step.Run(ctx)
			}
			return nil // Always stop after execution
		})
	}
}

// Reuse allows a step to be reused by calling it and then deciding what
// to do based on context state after execution.
func Reuse(
	stepToReuse NewStep,
	afterExecution func(context.Context) NewStep,
) NewStep {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute the reused step
			step := stepToReuse.Step()
			if step != nil {
				step.Run(ctx)
			}

			// Decide what to do next based on context state
			nextStep := afterExecution(ctx)
			if nextStep != nil {
				return nextStep(next).Run(ctx)
			}
			if next != nil {
				return next.Run(ctx)
			}
			return nil
		})
	}
}
