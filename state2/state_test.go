package state2

import (
	"context"
	"testing"
)

func TestHandler(t *testing.T) {
	t.Run("HandlerFunc implements Handler interface", func(t *testing.T) {
		var executed bool

		handler := HandlerFunc(func(ctx context.Context) Handler {
			executed = true
			return nil
		})

		result := handler.Handle(context.Background())

		if !executed {
			t.Error("Expected handler function to execute")
		}

		if result != nil {
			t.Error("Expected handler to return nil")
		}
	})
}

func TestNewHandler(t *testing.T) {
	t.Run("NewHandler.Handler() creates terminal handler", func(t *testing.T) {
		var executed bool

		newHandler := NewHandler(func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				executed = true
				// Should not call next since we're converting to terminal handler
				if next != nil {
					t.Error("Expected next to be nil for terminal handler")
				}
				return nil
			})
		})

		handler := newHandler.Handler()
		result := handler.Handle(context.Background())

		if !executed {
			t.Error("Expected handler to execute")
		}

		if result != nil {
			t.Error("Expected handler to return nil")
		}
	})

	t.Run("NewHandler chains with next handler", func(t *testing.T) {
		var executionOrder []string

		// Create a next handler
		nextHandler := HandlerFunc(func(ctx context.Context) Handler {
			executionOrder = append(executionOrder, "next")
			return nil
		})

		// Create a NewHandler that should call next
		newHandler := NewHandler(func(next Handler) Handler {
			return HandlerFunc(func(ctx context.Context) Handler {
				executionOrder = append(executionOrder, "current")
				if next != nil {
					return next.Handle(ctx)
				}
				return nil
			})
		})

		// Execute with next handler
		handler := newHandler(nextHandler)
		handler.Handle(context.Background())

		expectedOrder := []string{"current", "next"}
		if len(executionOrder) != len(expectedOrder) {
			t.Fatalf("Expected %d executions, got %d: %v", len(expectedOrder), len(executionOrder), executionOrder)
		}

		for i, expected := range expectedOrder {
			if executionOrder[i] != expected {
				t.Errorf("Expected execution[%d] = %q, got %q", i, expected, executionOrder[i])
			}
		}
	})
}

func TestStep(t *testing.T) {
	t.Run("Step executes function and terminates", func(t *testing.T) {
		var executed bool

		step := Step(func(ctx context.Context) {
			executed = true
		})

		handler := step.Handler()
		result := handler.Handle(context.Background())

		if !executed {
			t.Error("Expected step function to execute")
		}

		if result != nil {
			t.Error("Expected step to terminate (return nil)")
		}
	})

	t.Run("Step chains with next handler", func(t *testing.T) {
		var executionOrder []string

		// Create next handler
		nextHandler := HandlerFunc(func(ctx context.Context) Handler {
			executionOrder = append(executionOrder, "next")
			return nil
		})

		// Create step
		step := Step(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step")
		})

		// Chain them together
		handler := step(nextHandler)
		handler.Handle(context.Background())

		expectedOrder := []string{"step", "next"}
		if len(executionOrder) != len(expectedOrder) {
			t.Fatalf("Expected %d executions, got %d: %v", len(expectedOrder), len(executionOrder), executionOrder)
		}

		for i, expected := range expectedOrder {
			if executionOrder[i] != expected {
				t.Errorf("Expected execution[%d] = %q, got %q", i, expected, executionOrder[i])
			}
		}
	})

	t.Run("Multiple steps chain correctly", func(t *testing.T) {
		var executionOrder []string

		step1 := Step(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step1")
		})

		step2 := Step(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step2")
		})

		step3 := Step(func(ctx context.Context) {
			executionOrder = append(executionOrder, "step3")
		})

		// Chain: step1 -> step2 -> step3 -> nil
		handler := step1(step2(step3.Handler()))
		handler.Handle(context.Background())

		expectedOrder := []string{"step1", "step2", "step3"}
		if len(executionOrder) != len(expectedOrder) {
			t.Fatalf("Expected %d executions, got %d: %v", len(expectedOrder), len(executionOrder), executionOrder)
		}

		for i, expected := range expectedOrder {
			if executionOrder[i] != expected {
				t.Errorf("Expected execution[%d] = %q, got %q", i, expected, executionOrder[i])
			}
		}
	})

	t.Run("Step passes context through chain", func(t *testing.T) {
		const testKey = "test-key"
		const testValue = "test-value"

		var receivedValue string

		// First step adds value to context
		step1 := Step(func(ctx context.Context) {
			// In a real implementation, we'd create a new context with the value
			// For this test, we'll just verify the context is passed through
		})

		// Second step tries to read from context
		step2 := Step(func(ctx context.Context) {
			if value := ctx.Value(testKey); value != nil {
				receivedValue = value.(string)
			}
		})

		// Create context with test value
		ctx := context.WithValue(context.Background(), testKey, testValue)

		// Chain and execute
		handler := step1(step2.Handler())
		handler.Handle(ctx)

		if receivedValue != testValue {
			t.Errorf("Expected context value %q, got %q", testValue, receivedValue)
		}
	})
}

func TestContextPropagation(t *testing.T) {
	t.Run("context flows through handler chain", func(t *testing.T) {
		var contexts []context.Context

		// Handler that captures the context it receives
		captureContext := func(name string) NewHandler {
			return func(next Handler) Handler {
				return HandlerFunc(func(ctx context.Context) Handler {
					contexts = append(contexts, ctx)
					if next != nil {
						return next.Handle(ctx)
					}
					return nil
				})
			}
		}

		handler1 := captureContext("handler1")
		handler2 := captureContext("handler2")

		// Create initial context with a value
		initialCtx := context.WithValue(context.Background(), "key", "value")

		// Chain handlers
		chain := handler1(handler2.Handler())
		chain.Handle(initialCtx)

		if len(contexts) != 2 {
			t.Fatalf("Expected 2 contexts to be captured, got %d", len(contexts))
		}

		// Both handlers should receive the same context
		for i, ctx := range contexts {
			if ctx.Value("key") != "value" {
				t.Errorf("Handler %d did not receive expected context value", i+1)
			}
		}
	})
}
