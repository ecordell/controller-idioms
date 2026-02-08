package middleware_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

func TestRecursiveCircuitBreaker(t *testing.T) {
	var executionCount int
	var failureCount int

	m := middleware.RecursiveCircuitBreaker(2) // Open after 2 failures

	// Create a pipeline where we can control failures
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			executionCount++
		}),
		state.Action(func(ctx context.Context) {
			executionCount++
		}),
	)

	wrapped := state.WithMiddleware(pipeline, m)

	// First execution - cancel context to simulate failure
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1() // Cancel immediately to simulate error
	state.Run(ctx1, wrapped)
	failureCount++

	// Second execution - cancel context again
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	state.Run(ctx2, wrapped)
	failureCount++

	if failureCount != 2 {
		t.Errorf("expected 2 failures before circuit opens, got %d", failureCount)
	}

	// Third execution - circuit should be open, preventing execution
	beforeThird := executionCount
	state.Run(context.Background(), wrapped)
	afterThird := executionCount

	if afterThird != beforeThird {
		t.Errorf("third execution: expected circuit open (no new executions), but got %d new executions", afterThird-beforeThird)
	}
}
