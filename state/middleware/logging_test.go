package middleware_test

import (
	"context"
	"strings"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/middleware"
)

// Simple logger for testing
type testLogger struct {
	logs []string
}

func (l *testLogger) Log(msg string) {
	l.logs = append(l.logs, msg)
}

func TestRecursiveLogging(t *testing.T) {
	logger := &testLogger{}

	m := middleware.RecursiveLogging(logger.Log)

	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	wrapped := state.WithMiddleware(pipeline, m)
	state.Run(context.Background(), wrapped)

	// Should have logged before/after each step (6 logs minimum)
	if len(logger.logs) < 6 {
		t.Errorf("expected at least 6 log entries, got %d: %v", len(logger.logs), logger.logs)
	}

	// Verify "executing step" appears in logs
	hasExecutingStep := false
	for _, log := range logger.logs {
		if strings.Contains(log, "executing step") || strings.Contains(log, "completed step") {
			hasExecutingStep = true
			break
		}
	}

	if !hasExecutingStep {
		t.Error("expected 'executing step' or 'completed step' in logs")
	}
}
