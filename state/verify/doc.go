// Package verify provides model checking utilities for state pipelines.
//
// This package enables property verification through execution tracing
// and runtime analysis. Properties that can be verified include:
//
// - Safety properties: "Bad things never happen"
// - Liveness properties: "Good things eventually happen"
// - Temporal properties: "Events occur in expected order"
//
// # Execution Tracing
//
// The ExecutionTrace captures the complete execution path through a pipeline:
//
//	tracer, trace := verify.NewTracer()
//	pipeline := state.Sequence(step1, step2, step3)
//	wrapped := state.WithMiddleware(pipeline, tracer)
//	state.Run(ctx, wrapped)
//
//	// Analyze the trace
//	fmt.Printf("Executed %d steps\n", len(trace.Steps))
//	for i, step := range trace.Steps {
//		fmt.Printf("Step %d: took %v\n", i, step.Duration)
//	}
//
// # Property Verification
//
// Verify properties about pipeline execution:
//
//	// Safety: No panics occurred
//	err := verify.VerifyNoPanic(pipeline)
//	if err != nil {
//		log.Fatal("Pipeline panicked:", err)
//	}
//
//	// Liveness: Pipeline terminates within timeout
//	err := verify.VerifyTerminates(pipeline, 5*time.Second)
//	if err != nil {
//		log.Fatal("Pipeline did not terminate:", err)
//	}
//
//	// Progress: No infinite loops (max 100 steps)
//	err := verify.VerifyProgress(pipeline, 100)
//	if err != nil {
//		log.Fatal("Pipeline exceeded max steps:", err)
//	}
//
//	// Temporal: Steps occur in expected order
//	err := verify.VerifyOrdering(pipeline, "auth", "process")
//
// # Integration with Middleware
//
// Verification works seamlessly with middleware from state/middleware:
//
//	import (
//		"github.com/authzed/controller-idioms/state/middleware"
//		"github.com/authzed/controller-idioms/state/verify"
//	)
//
//	// Create pipeline with logging and rate limiting
//	pipeline := state.Sequence(step1, step2, step3)
//
//	// Add production middleware
//	wrapped := state.WithMiddleware(
//		pipeline,
//		middleware.RecursiveLogging(logger),
//		middleware.RecursiveRateLimit(limiter),
//	)
//
//	// Verify it works correctly
//	if err := verify.VerifyNoPanic(wrapped); err != nil {
//		log.Fatal("Pipeline with middleware panicked:", err)
//	}
//
// # Complete Example
//
// A complete example of verifying a complex pipeline:
//
//	func TestMyPipeline(t *testing.T) {
//		// Define your pipeline
//		pipeline := state.Sequence(
//			state.Action(authenticateUser),
//			state.Decision(
//				isAuthorized,
//				state.Action(processRequest),
//				state.Action(rejectRequest),
//			),
//		)
//
//		// Run all verifications
//		t.Run("NoPanic", func(t *testing.T) {
//			if err := verify.VerifyNoPanic(pipeline); err != nil {
//				t.Fatal(err)
//			}
//		})
//
//		t.Run("Terminates", func(t *testing.T) {
//			if err := verify.VerifyTerminates(pipeline, 1*time.Second); err != nil {
//				t.Fatal(err)
//			}
//		})
//
//		t.Run("Progress", func(t *testing.T) {
//			if err := verify.VerifyProgress(pipeline, 10); err != nil {
//				t.Fatal(err)
//			}
//		})
//	}
package verify
