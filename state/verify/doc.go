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
//
// # Property Verification
//
// Verify properties about pipeline execution:
//
//	// Safety: No panics occurred
//	err := verify.VerifyNoPanic(pipeline)
//
//	// Liveness: Pipeline terminates within timeout
//	err := verify.VerifyTerminates(pipeline, 5*time.Second)
//
//	// Temporal: Steps occur in expected order
//	err := verify.VerifyOrdering(pipeline, "auth", "process")
package verify
