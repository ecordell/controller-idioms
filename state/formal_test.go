package state

import (
	"context"
	"testing"
)

// =============================================================================
// CATEGORY THEORY TESTS
// =============================================================================

func TestCategoryIdentityLaw(t *testing.T) {
	ctx := context.WithValue(context.Background(), "test", "value")

	// Test morphism
	transform := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "transformed", true)
	}

	id := Identity()

	// Left identity: id ∘ f = f
	leftCompose := Compose(id, transform)
	leftResult := leftCompose(ctx)

	// Right identity: f ∘ id = f
	rightCompose := Compose(transform, id)
	rightResult := rightCompose(ctx)

	// Direct application
	directResult := transform(ctx)

	// All should be equivalent (checking structure)
	if leftResult.Value("transformed") != directResult.Value("transformed") {
		t.Error("Left identity law violated")
	}

	if rightResult.Value("transformed") != directResult.Value("transformed") {
		t.Error("Right identity law violated")
	}

	t.Log("Category identity laws verified")
}

func TestCategoryAssociativityLaw(t *testing.T) {
	ctx := context.Background()

	f := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "f", true)
	}

	g := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "g", true)
	}

	h := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "h", true)
	}

	// (h ∘ g) ∘ f
	left := Compose(f, Compose(g, h))
	leftResult := left(ctx)

	// h ∘ (g ∘ f)
	right := Compose(Compose(f, g), h)
	rightResult := right(ctx)

	// Both should have all three values set
	if leftResult.Value("f") == nil || leftResult.Value("g") == nil || leftResult.Value("h") == nil {
		t.Error("Left associativity composition failed")
	}

	if rightResult.Value("f") == nil || rightResult.Value("g") == nil || rightResult.Value("h") == nil {
		t.Error("Right associativity composition failed")
	}

	t.Log("Category associativity law verified")
}

// =============================================================================
// KLEISLI CATEGORY TESTS
// =============================================================================

func TestKleisliComposition(t *testing.T) {
	ctx := context.Background()
	var results []string

	// First Kleisli arrow
	stage1 := Do(func(ctx context.Context) context.Context {
		results = append(results, "stage1")
		return context.WithValue(ctx, "user", "alice")
	})

	// Second Kleisli arrow
	stage2 := Do(func(ctx context.Context) context.Context {
		user := ctx.Value("user").(string)
		results = append(results, "stage2-"+user)
		return context.WithValue(ctx, "processed", true)
	})

	// Compose using our Kleisli composition
	composed := KleisliCompose(stage1, stage2)

	// Execute
	Run(ctx, composed)

	expected := []string{"stage1", "stage2-alice"}
	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}

	t.Log("Kleisli composition verified")
}

func TestKleisliIdentity(t *testing.T) {
	ctx := context.Background()
	var executed bool

	testStage := Do(func(ctx context.Context) context.Context {
		executed = true
		return context.WithValue(ctx, "test", "value")
	})

	// Identity should not change behavior
	identityStage := Unit(Identity())

	// Compose with identity (should be equivalent to original)
	leftComposed := KleisliCompose(identityStage, testStage)
	rightComposed := KleisliCompose(testStage, identityStage)

	// Test left identity
	executed = false
	Run(ctx, leftComposed)
	if !executed {
		t.Error("Left Kleisli identity failed")
	}

	// Test right identity
	executed = false
	Run(ctx, rightComposed)
	if !executed {
		t.Error("Right Kleisli identity failed")
	}

	t.Log("Kleisli identity laws verified")
}

func TestKleisliAssociativity(t *testing.T) {
	ctx := context.Background()
	var results []string

	f := Do(func(ctx context.Context) context.Context {
		results = append(results, "f")
		return ctx
	})

	g := Do(func(ctx context.Context) context.Context {
		results = append(results, "g")
		return ctx
	})

	h := Do(func(ctx context.Context) context.Context {
		results = append(results, "h")
		return ctx
	})

	// (h ∘ g) ∘ f
	left := KleisliCompose(f, KleisliCompose(g, h))

	// h ∘ (g ∘ f)
	right := KleisliCompose(KleisliCompose(f, g), h)

	// Both should execute in the same order: f, g, h
	results = nil
	Run(ctx, left)
	leftResults := make([]string, len(results))
	copy(leftResults, results)

	results = nil
	Run(ctx, right)
	rightResults := make([]string, len(results))
	copy(rightResults, results)

	if len(leftResults) != 3 || len(rightResults) != 3 {
		t.Fatal("Associativity test failed - wrong number of executions")
	}

	for i := 0; i < 3; i++ {
		if leftResults[i] != rightResults[i] {
			t.Errorf("Associativity failed at position %d: left=%q, right=%q",
				i, leftResults[i], rightResults[i])
		}
	}

	t.Log("Kleisli associativity law verified")
}

// =============================================================================
// FUNCTOR LAWS TESTS
// =============================================================================

func TestFunctorIdentityLaw(t *testing.T) {
	ctx := context.Background()
	var executed bool

	baseStage := Do(func(ctx context.Context) context.Context {
		executed = true
		return ctx
	})

	// fmap id = id
	mappedStage := MapF(Identity(), baseStage)

	// Should behave the same as the original stage
	executed = false
	Run(ctx, mappedStage)
	if !executed {
		t.Error("Functor identity law failed")
	}

	t.Log("Functor identity law verified")
}

func TestFunctorCompositionLaw(t *testing.T) {
	ctx := context.Background()
	var results []string
	var finalValue int

	baseStage := Do(func(ctx context.Context) context.Context {
		results = append(results, "base")
		return context.WithValue(ctx, "value", 0)
	})

	f := func(ctx context.Context) context.Context {
		if val := ctx.Value("value"); val != nil {
			return context.WithValue(ctx, "value", val.(int)+1)
		}
		return context.WithValue(ctx, "value", 1)
	}

	g := func(ctx context.Context) context.Context {
		if val := ctx.Value("value"); val != nil {
			return context.WithValue(ctx, "value", val.(int)*2)
		}
		return context.WithValue(ctx, "value", 0)
	}

	// Helper to extract final value
	extractValue := Do(func(ctx context.Context) context.Context {
		if val := ctx.Value("value"); val != nil {
			finalValue = val.(int)
		}
		return ctx
	})

	// fmap (g ∘ f) = fmap g ∘ fmap f
	left := Sequence(MapF(Compose(f, g), baseStage), extractValue)

	// Reset and test
	results = nil
	finalValue = 0
	Run(ctx, left)
	leftValue := finalValue

	// Test right side
	right := Sequence(MapF(g, MapF(f, baseStage)), extractValue)
	results = nil
	finalValue = 0
	Run(ctx, right)
	rightValue := finalValue

	// Both should produce the same final value
	if leftValue != rightValue {
		t.Errorf("Functor composition law failed: left=%d, right=%d", leftValue, rightValue)
	}

	t.Log("Functor composition law verified")
}

// =============================================================================
// MONAD LAWS TESTS
// =============================================================================

func TestMonadLeftIdentityLaw(t *testing.T) {
	ctx := context.Background()
	var results []string

	// Pure value lifted into monad
	pureTransform := func(ctx context.Context) context.Context {
		results = append(results, "pure")
		return context.WithValue(ctx, "user", "alice")
	}

	// Function that takes the value and returns a monadic computation
	k := func() KleisliArrow {
		return Do(func(ctx context.Context) context.Context {
			user := ctx.Value("user").(string)
			results = append(results, "k-"+user)
			return ctx
		})
	}

	// return a >>= k should equal k a
	// In our case: Unit(pureTransform) composed with k()
	left := KleisliCompose(Unit(pureTransform), k())

	// Execute and verify
	results = nil
	Run(ctx, left)

	expected := []string{"pure", "k-alice"}
	if len(results) != len(expected) {
		t.Fatalf("Left identity: expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Left identity: expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}

	t.Log("Monad left identity law verified")
}

func TestMonadRightIdentityLaw(t *testing.T) {
	ctx := context.Background()
	var results []string

	// Monadic computation
	m := Do(func(ctx context.Context) context.Context {
		results = append(results, "m")
		return context.WithValue(ctx, "test", "value")
	})

	// m >>= return should equal m
	// In our case: Sequence with identity
	bound := KleisliCompose(m, Unit(Identity()))

	// Both should produce the same observable behavior
	results = nil
	Run(ctx, m)
	if len(results) == 0 || results[0] != "m" {
		t.Error("Original stage did not execute properly")
	}

	results = nil
	Run(ctx, bound)
	if len(results) == 0 || results[0] != "m" {
		t.Error("Bound stage did not execute properly")
	}

	t.Log("Monad right identity law verified")
}

// =============================================================================
// PRACTICAL COMBINATORS TESTS
// =============================================================================

func TestSequencePreservesComposition(t *testing.T) {
	ctx := context.Background()
	var results []string

	stage1 := Do(func(ctx context.Context) context.Context {
		results = append(results, "1")
		return context.WithValue(ctx, "count", 1)
	})

	stage2 := Do(func(ctx context.Context) context.Context {
		count := ctx.Value("count").(int)
		results = append(results, "2")
		return context.WithValue(ctx, "count", count+1)
	})

	stage3 := Do(func(ctx context.Context) context.Context {
		count := ctx.Value("count").(int)
		results = append(results, "3")
		return context.WithValue(ctx, "count", count+1)
	})

	// Nested sequences should be equivalent to flat sequence
	nested := SequenceC(stage1, SequenceC(stage2, stage3))
	flat := SequenceC(stage1, stage2, stage3)

	// Test nested
	results = nil
	Run(ctx, nested)
	nestedResults := make([]string, len(results))
	copy(nestedResults, results)

	// Test flat
	results = nil
	Run(ctx, flat)
	flatResults := make([]string, len(results))
	copy(flatResults, results)

	// Should produce same execution order
	if len(nestedResults) != len(flatResults) {
		t.Fatal("Sequence associativity failed: different lengths")
	}

	for i := 0; i < len(nestedResults); i++ {
		if nestedResults[i] != flatResults[i] {
			t.Errorf("Sequence associativity failed at position %d: nested=%q, flat=%q",
				i, nestedResults[i], flatResults[i])
		}
	}

	expected := []string{"1", "2", "3"}
	for i, exp := range expected {
		if flatResults[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, flatResults[i])
		}
	}

	t.Log("Sequence composition laws verified")
}

func TestDecisionPreservesStructure(t *testing.T) {
	ctx := context.WithValue(context.Background(), "condition", true)
	var results []string

	trueStage := Do(func(ctx context.Context) context.Context {
		results = append(results, "true-branch")
		return ctx
	})

	falseStage := Do(func(ctx context.Context) context.Context {
		results = append(results, "false-branch")
		return ctx
	})

	predicate := func(ctx context.Context) bool {
		return ctx.Value("condition").(bool)
	}

	decision := ChoiceC(predicate, trueStage, falseStage)

	// Should execute true branch
	results = nil
	Run(ctx, decision)

	if len(results) != 1 || results[0] != "true-branch" {
		t.Errorf("Decision failed: expected [true-branch], got %v", results)
	}

	// Test false condition
	ctx = context.WithValue(context.Background(), "condition", false)
	results = nil
	Run(ctx, decision)

	if len(results) != 1 || results[0] != "false-branch" {
		t.Errorf("Decision failed: expected [false-branch], got %v", results)
	}

	t.Log("Decision structure preservation verified")
}

// =============================================================================
// NATURAL TRANSFORMATION TESTS
// =============================================================================

func TestNaturalTransformationProperty(t *testing.T) {
	ctx := context.Background()

	// Base stage that sets a value
	baseStage := Do(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "value", 42)
	})

	// Morphism that transforms the value
	transform := func(ctx context.Context) context.Context {
		if val := ctx.Value("value"); val != nil {
			return context.WithValue(ctx, "value", val.(int)*2)
		}
		return ctx
	}

	// Natural transformation property:
	// Transform then extract should equal extract then transform

	extract := func(ctx context.Context) (int, error) {
		if val := ctx.Value("value"); val != nil {
			return val.(int), nil
		}
		return 0, nil
	}

	// Path 1: Transform the stage, then convert to Option
	transformedStage := MapF(transform, baseStage)
	option1 := ToOption(transformedStage, extract)
	result1 := option1(ctx)

	// Path 2: Convert to Option, then transform the result
	option2 := ToOption(baseStage, extract)
	baseResult := option2(ctx)

	// Transform the result
	var result2 Option[int]
	if baseResult.Value != nil {
		transformedValue := *baseResult.Value * 2
		result2 = Option[int]{Value: &transformedValue}
	}

	// Both paths should yield equivalent results
	if result1.Value == nil || result2.Value == nil {
		t.Fatal("Natural transformation test failed: nil values")
	}

	if *result1.Value != *result2.Value {
		t.Errorf("Natural transformation property failed: %d != %d",
			*result1.Value, *result2.Value)
	}

	t.Log("Natural transformation properties verified")
}

// =============================================================================
// INTERPRETER TESTS
// =============================================================================

func TestDirectInterpreter(t *testing.T) {
	ctx := context.Background()

	stage := Do(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "interpreted", true)
	})

	interpreter := DirectInterpreter{}
	result := interpreter.Interpret(ctx, stage)

	if result.Value("interpreted") == nil {
		t.Error("Direct interpreter failed to execute stage")
	}

	t.Log("Direct interpreter verified")
}

func TestTracingInterpreter(t *testing.T) {
	ctx := context.Background()

	stage := Do(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "traced", true)
	})

	interpreter := &TracingInterpreter{}
	result := interpreter.Interpret(ctx, stage)

	if len(interpreter.Trace) == 0 {
		t.Error("Tracing interpreter failed to record trace")
	}

	if result.Value("traced") == nil {
		t.Error("Tracing interpreter failed to execute stage")
	}

	t.Log("Tracing interpreter verified")
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestCompleteCategory(t *testing.T) {
	// This test verifies that our complete system works as a proper category
	// by building a complex pipeline and verifying it behaves correctly

	ctx := context.Background()
	var trace []string

	// Build a complex pipeline using all our combinators
	pipeline := SequenceC(
		// Step 1: Initialize
		Do(func(ctx context.Context) context.Context {
			trace = append(trace, "init")
			return context.WithValue(ctx, "step", 1)
		}),

		// Step 2: Conditional branching
		ChoiceC(
			func(ctx context.Context) bool {
				return ctx.Value("step").(int) == 1
			},
			// True branch: continue processing
			SequenceC(
				Do(func(ctx context.Context) context.Context {
					trace = append(trace, "branch-true")
					return context.WithValue(ctx, "step", 2)
				}),
				// Parallel processing
				ParallelC(
					Do(func(ctx context.Context) context.Context {
						trace = append(trace, "parallel-1")
						return ctx
					}),
					Do(func(ctx context.Context) context.Context {
						trace = append(trace, "parallel-2")
						return ctx
					}),
				),
			),
			// False branch: error handling
			Do(func(ctx context.Context) context.Context {
				trace = append(trace, "branch-false")
				return ctx
			}),
		),

		// Step 3: Finalization
		Do(func(ctx context.Context) context.Context {
			trace = append(trace, "finalize")
			return ctx
		}),
	)

	// Execute the complete pipeline
	Run(ctx, pipeline)

	// Verify execution trace
	expectedMin := []string{"init", "branch-true", "finalize"}

	// Check minimum expected elements (parallel execution order may vary)
	for _, expected := range expectedMin {
		found := false
		for _, actual := range trace {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing expected trace element: %s in %v", expected, trace)
		}
	}

	// Should have parallel executions too
	parallel1Found := false
	parallel2Found := false
	for _, item := range trace {
		if item == "parallel-1" {
			parallel1Found = true
		}
		if item == "parallel-2" {
			parallel2Found = true
		}
	}

	if !parallel1Found || !parallel2Found {
		t.Error("Parallel execution did not occur properly")
	}

	t.Logf("Complete category integration test passed. Trace: %v", trace)
}
