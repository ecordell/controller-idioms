package transform

import (
	"context"

	"github.com/authzed/controller-idioms/state"
)

type contextKey int

const transformsKey contextKey = 0

// WithTransform registers a middleware transform in the context.
// Multiple transforms can be registered by calling this multiple times.
// Transforms are applied in the order they are registered.
func WithTransform(ctx context.Context, transform state.Middleware) context.Context {
	transforms := GetTransforms(ctx)
	if transforms == nil {
		transforms = []state.Middleware{}
	}
	transforms = append(transforms, transform)
	return context.WithValue(ctx, transformsKey, transforms)
}

// GetTransforms retrieves all middleware transforms from the context.
// Returns nil if no transforms are registered.
func GetTransforms(ctx context.Context) []state.Middleware {
	if v := ctx.Value(transformsKey); v != nil {
		return v.([]state.Middleware)
	}
	return nil
}
