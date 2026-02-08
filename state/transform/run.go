package transform

import (
	"context"

	"github.com/authzed/controller-idioms/state"
)

// RunWithTransforms executes a pipeline with all middleware transforms
// registered in the context automatically applied.
//
// Transforms are applied in the order they were registered with WithTransform.
// This is equivalent to manually calling state.WithMiddleware for each
// transform and then calling state.Run.
func RunWithTransforms(ctx context.Context, pipeline state.NewStep) {
	transforms := GetTransforms(ctx)
	if len(transforms) > 0 {
		pipeline = state.WithMiddleware(pipeline, transforms...)
	}
	state.Run(ctx, pipeline)
}
