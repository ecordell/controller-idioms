// Package transform provides utilities for creating recursive middleware
// that propagates through continuation chains in state pipelines.
//
// The key insight is that middleware can wrap not just the current step,
// but also the next continuation and any returned steps, allowing
// transformations to "thread through" entire pipeline executions.
package transform
