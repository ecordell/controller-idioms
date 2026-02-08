package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestWithTransform(t *testing.T) {
	ctx := context.Background()

	middleware := func(s state.NewStep) state.NewStep {
		return s
	}

	ctx = transform.WithTransform(ctx, middleware)

	transforms := transform.GetTransforms(ctx)
	if len(transforms) != 1 {
		t.Errorf("expected 1 transform, got %d", len(transforms))
	}
}

func TestWithTransformMultiple(t *testing.T) {
	ctx := context.Background()

	m1 := func(s state.NewStep) state.NewStep { return s }
	m2 := func(s state.NewStep) state.NewStep { return s }
	m3 := func(s state.NewStep) state.NewStep { return s }

	ctx = transform.WithTransform(ctx, m1)
	ctx = transform.WithTransform(ctx, m2)
	ctx = transform.WithTransform(ctx, m3)

	transforms := transform.GetTransforms(ctx)
	if len(transforms) != 3 {
		t.Errorf("expected 3 transforms, got %d", len(transforms))
	}
}

func TestGetTransformsEmpty(t *testing.T) {
	ctx := context.Background()

	transforms := transform.GetTransforms(ctx)
	if transforms != nil {
		t.Errorf("expected nil for empty context, got %v", transforms)
	}
}
