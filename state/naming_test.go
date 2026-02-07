package state

import (
	"context"
	"testing"
)

func TestHandlerNewStepStepNaming(t *testing.T) {
	ctx := context.Background()
	var executed []string

	// Test that Do() creates context-transforming steps
	addUser := Do(func(ctx context.Context) context.Context {
		executed = append(executed, "adding-user")
		return context.WithValue(ctx, "user", "alice")
	})

	readUser := Do(func(ctx context.Context) context.Context {
		if user := ctx.Value("user"); user != nil {
			executed = append(executed, "user-is-"+user.(string))
		}
		return ctx
	})

	// Test that NewStep is the constructor type
	var constructor NewStep = addUser
	_ = constructor

	// Test that Step is the execution interface
	var step Step = addUser.Step()
	_ = step

	// Test that composition works
	pipeline := Sequence(addUser, readUser)
	Run(ctx, pipeline)

	expected := []string{"adding-user", "user-is-alice"}
	if len(executed) != len(expected) {
		t.Fatalf("Expected %d executions, got %d: %v", len(expected), len(executed), executed)
	}

	for i, exp := range expected {
		if executed[i] != exp {
			t.Errorf("Expected executed[%d] = %q, got %q", i, exp, executed[i])
		}
	}

	t.Log("Step/NewStep/Do naming works correctly!")
}

func TestContextThreadingWithNewNaming(t *testing.T) {
	ctx := context.Background()
	var values []string

	pipeline := Sequence(
		Do(func(ctx context.Context) context.Context {
			values = append(values, "step1")
			return context.WithValue(ctx, "count", 1)
		}),
		Do(func(ctx context.Context) context.Context {
			count := ctx.Value("count").(int)
			values = append(values, "step2-count-1")
			return context.WithValue(ctx, "count", count+1)
		}),
		Do(func(ctx context.Context) context.Context {
			_ = ctx.Value("count").(int) // Read but don't store in variable
			values = append(values, "step3-count-2")
			return ctx
		}),
	)

	Run(ctx, pipeline)

	expected := []string{"step1", "step2-count-1", "step3-count-2"}
	if len(values) != len(expected) {
		t.Fatalf("Expected %d values, got %d: %v", len(expected), len(values), values)
	}

	for i, exp := range expected {
		if values[i] != exp {
			t.Errorf("Expected values[%d] = %q, got %q", i, exp, values[i])
		}
	}

	t.Log("Context threading works with new naming!")
}
