package verify

import (
	"context"
	"sync"
	"time"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

// ExecutionTrace records the execution path through a pipeline.
type ExecutionTrace struct {
	Steps []StepTrace
	mu    sync.Mutex
}

// StepTrace records information about a single step execution.
type StepTrace struct {
	StepID    int       // Unique step identifier
	StartTime time.Time // When step started
	EndTime   time.Time // When step completed
	Duration  time.Duration
	Panicked  bool  // Whether step panicked
	Error     error // Context error if any
}

// NewTracer creates a tracing middleware and returns both the middleware
// and the trace it populates.
func NewTracer() (state.Middleware, *ExecutionTrace) {
	trace := &ExecutionTrace{
		Steps: []StepTrace{},
	}

	stepCounter := 0

	middleware := transform.RecursiveMiddleware(
		func(ctx context.Context) {
			trace.mu.Lock()
			stepCounter++
			stepID := stepCounter
			trace.Steps = append(trace.Steps, StepTrace{
				StepID:    stepID,
				StartTime: time.Now(),
			})
			trace.mu.Unlock()
		},
		func(ctx context.Context) {
			trace.mu.Lock()
			if len(trace.Steps) > 0 {
				lastIdx := len(trace.Steps) - 1
				trace.Steps[lastIdx].EndTime = time.Now()
				trace.Steps[lastIdx].Duration = trace.Steps[lastIdx].EndTime.Sub(trace.Steps[lastIdx].StartTime)
				trace.Steps[lastIdx].Error = ctx.Err()
			}
			trace.mu.Unlock()
		},
	)

	return middleware, trace
}
