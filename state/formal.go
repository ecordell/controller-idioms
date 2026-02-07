// Package formal provides the mathematical foundations for the Step system.
//
// The Step system forms a Kleisli category where:
// - Objects are Context states
// - Morphisms are context transformations (Context -> Context)
// - Kleisli arrows are Steps (Context -> (Context, Continuation))
// - Composition is provided by Sequence, Decision, etc.
//
// This provides a formal foundation for compositional computation with context threading.
package state

import "context"

// =============================================================================
// CATEGORY THEORY FOUNDATIONS
// =============================================================================

// Object represents an object in our category - a context state.
type Object = context.Context

// Morphism represents a morphism in our category - a pure context transformation.
// In category theory terms: f: A -> B where A and B are context states.
type Morphism = func(context.Context) context.Context

// Identity morphism - the identity transformation for any context.
func Identity() Morphism {
	return func(ctx context.Context) context.Context {
		return ctx
	}
}

// Compose composes two morphisms.
// In category theory: if f: A -> B and g: B -> C, then g ∘ f: A -> C
func Compose(f, g Morphism) Morphism {
	return func(ctx context.Context) context.Context {
		return g(f(ctx))
	}
}

// =============================================================================
// KLEISLI CATEGORY
// =============================================================================

// KleisliArrow represents a Kleisli arrow in our "Step monad".
// Conceptually: Context -> M(Context) where M is our "Step monad"
// Practically: This is what NewStep represents.
type KleisliArrow = NewStep

// KleisliCompose composes two Kleisli arrows.
// This is the fundamental operation that makes stages composable.
// In Haskell notation: (>=>) :: (a -> m b) -> (b -> m c) -> (a -> m c)
func KleisliCompose(f, g KleisliArrow) KleisliArrow {
	return Sequence(f, g)
}

// =============================================================================
// FORMAL HANDLER MONAD
// =============================================================================

// The Step type forms a monad with the following operations:

// Unit (η): Morphism -> KleisliArrow
// Lifts a pure context transformation into the Step monad.
func Unit(m Morphism) KleisliArrow {
	return Do(m)
}

// Join (μ): Would flatten nested stages, but our continuation-passing style
// handles this automatically through the next parameter.

// =============================================================================
// FUNCTOR OPERATIONS
// =============================================================================

// MapF lifts a morphism to operate on steps.
// This is the functor operation: fmap :: (a -> b) -> f a -> f b
func MapF(m Morphism, step KleisliArrow) KleisliArrow {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			transformedCtx := m(ctx)
			wrappedStep := step(next)
			if wrappedStep != nil {
				return wrappedStep.Run(transformedCtx)
			}
			if next != nil {
				return next.Run(transformedCtx)
			}
			return nil
		})
	}
}

// =============================================================================
// CATEGORY LAWS VERIFICATION
// =============================================================================

// These functions can be used in tests to verify our category laws hold.

// VerifyIdentityLaw verifies that Identity is indeed the identity morphism.
// For any morphism f: f ∘ id = id ∘ f = f
func VerifyIdentityLaw(f Morphism, ctx context.Context) bool {
	id := Identity()

	// f ∘ id = f
	leftIdentity := Compose(id, f)(ctx)
	direct := f(ctx)

	// id ∘ f = f
	rightIdentity := Compose(f, id)(ctx)

	// In practice, we'd need deep equality comparison for contexts
	// This is a structural check
	return leftIdentity == direct && rightIdentity == direct
}

// VerifyAssociativityLaw verifies morphism composition is associative.
// For morphisms f, g, h: (h ∘ g) ∘ f = h ∘ (g ∘ f)
func VerifyAssociativityLaw(f, g, h Morphism, ctx context.Context) bool {
	// (h ∘ g) ∘ f
	left := Compose(f, Compose(g, h))(ctx)

	// h ∘ (g ∘ f)
	right := Compose(Compose(f, g), h)(ctx)

	return left == right
}

// =============================================================================
// MONAD LAWS
// =============================================================================

// The monad laws for our Step system are:
//
// Left Identity:  Unit(a) >>= f  ≡  f(a)
//   Do(id) composed with f should be equivalent to just f
//
// Right Identity:  m >>= Unit  ≡  m
//   Composing with Do(id) should not change behavior
//
// Associativity:  (m >>= f) >>= g  ≡  m >>= (\x -> f(x) >>= g)
//   Order of composition operations doesn't matter
//
// These laws are verified through the test suite in formal_test.go by:
// 1. Testing that Do(Identity()) composes correctly (left/right identity)
// 2. Testing that Sequence composition is associative
// 3. Testing that Bind operations compose correctly
//
// Note: Runtime verification of these laws for arbitrary steps would require:
// - Executing steps and capturing/comparing their effects
// - Deep equality comparison of context modifications
// - Formal proof techniques or property-based testing
//
// The structural tests in formal_test.go provide confidence that the laws hold
// for the concrete implementations we provide.

// =============================================================================
// FORMAL COMBINATORS
// =============================================================================

// These are the fundamental combinators that preserve the categorical structure.

// SequenceC is the categorical product for steps - sequential composition.
func SequenceC(steps ...KleisliArrow) KleisliArrow {
	return Sequence(steps...)
}

// ChoiceC is the categorical coproduct for steps - choice composition.
func ChoiceC(predicate func(context.Context) bool, left, right KleisliArrow) KleisliArrow {
	return Decision(predicate, left, right)
}

// ParallelC represents parallel composition in our category.
func ParallelC(steps ...KleisliArrow) KleisliArrow {
	return Parallel(steps...)
}

// =============================================================================
// NATURAL TRANSFORMATIONS
// =============================================================================

// A natural transformation between our Step "functor" and other functors.

// ToOption transforms a step that might fail into an optional result.
// This demonstrates how our Step system can interface with other monadic systems.
type Option[T any] struct {
	Value *T
	Error error
}

func ToOption[T any](step KleisliArrow, extract func(context.Context) (T, error)) func(context.Context) Option[T] {
	return func(ctx context.Context) Option[T] {
		// Execute the step
		s := step(nil)
		if s != nil {
			s.Run(ctx)
		}

		// Extract the result
		value, err := extract(ctx)
		if err != nil {
			return Option[T]{Error: err}
		}
		return Option[T]{Value: &value}
	}
}

// =============================================================================
// HIGHER-ORDER OPERATIONS
// =============================================================================

// Lift lifts a function of arity n to work on steps.
// This is a generalization of Map for multiple arguments.

// Lift2 lifts a binary function to work on two steps.
func Lift2(f func(context.Context, context.Context) context.Context, handler1, handler2 KleisliArrow) KleisliArrow {
	return func(next Step) Step {
		return StepFunc(func(ctx context.Context) Step {
			// Execute first step
			s1 := handler1(nil)
			var ctx1 context.Context = ctx
			if s1 != nil {
				s1.Run(ctx1) // This modifies ctx1 through side effects conceptually
			}

			// Execute second step
			s2 := handler2(nil)
			var ctx2 context.Context = ctx
			if s2 != nil {
				s2.Run(ctx2)
			}

			// Combine results
			result := f(ctx1, ctx2)

			if next != nil {
				return next.Run(result)
			}
			return nil
		})
	}
}

// =============================================================================
// ALGEBRAIC STRUCTURE
// =============================================================================

// Our Step system forms several algebraic structures:

// 1. Category: Objects (contexts), Morphisms (transformations), Composition
// 2. Kleisli Category: Based on our "Step monad"
// 3. Monad: Unit (Step), Bind (monadic composition), with laws
// 4. Applicative Functor: Apply operations (Lift2, etc.)
// 5. Functor: Map operation

// This provides a solid mathematical foundation for:
// - Compositional reasoning about step pipelines
// - Formal verification of step behavior
// - Type-safe step construction and combination
// - Lawful behavior guarantees

// =============================================================================
// INTERPRETATIONS
// =============================================================================

// Different ways to interpret/execute our step descriptions:

// DirectInterpreter executes steps immediately.
type DirectInterpreter struct{}

func (DirectInterpreter) Interpret(ctx context.Context, step KleisliArrow) context.Context {
	// Create a wrapper that captures the final context
	var finalCtx context.Context

	wrapper := StepFunc(func(receivedCtx context.Context) Step {
		finalCtx = receivedCtx
		return nil
	})

	s := step(wrapper)
	if s != nil {
		s.Run(ctx)
	}

	if finalCtx != nil {
		return finalCtx
	}
	return ctx
}

// TracingInterpreter executes steps while building a trace.
type TracingInterpreter struct {
	Trace []string
}

func (t *TracingInterpreter) Interpret(ctx context.Context, step KleisliArrow) context.Context {
	t.Trace = append(t.Trace, "executing step")

	// Create a wrapper that captures the final context
	var finalCtx context.Context

	wrapper := StepFunc(func(receivedCtx context.Context) Step {
		finalCtx = receivedCtx
		return nil
	})

	s := step(wrapper)
	if s != nil {
		s.Run(ctx)
	}

	if finalCtx != nil {
		return finalCtx
	}
	return ctx
}

// This separation of description and interpretation enables:
// - Testing with mock interpreters
// - Tracing and debugging
// - Optimization through different execution strategies
// - Formal analysis of step behavior
