package transform_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/transform"
)

func TestRunWithTransforms(t *testing.T) {
	var executed bool
	pipeline := state.Action(func(ctx context.Context) {
		executed = true
	})

	ctx := context.Background()
	transform.RunWithTransforms(ctx, pipeline)

	if !executed {
		t.Error("pipeline was not executed")
	}
}

func TestRunWithTransformsAppliesMiddleware(t *testing.T) {
	var beforeCalled, actionCalled bool

	before := func(ctx context.Context) {
		beforeCalled = true
	}

	middleware := transform.RecursiveMiddleware(before, nil)

	pipeline := state.Action(func(ctx context.Context) {
		actionCalled = true
	})

	ctx := context.Background()
	ctx = transform.WithTransform(ctx, middleware)

	transform.RunWithTransforms(ctx, pipeline)

	if !beforeCalled {
		t.Error("middleware before function was not called")
	}
	if !actionCalled {
		t.Error("pipeline action was not called")
	}
}

func TestRunWithTransformsMultipleMiddleware(t *testing.T) {
	var m1Called, m2Called, actionCalled bool

	m1 := transform.RecursiveMiddleware(
		func(ctx context.Context) { m1Called = true },
		nil,
	)

	m2 := transform.RecursiveMiddleware(
		func(ctx context.Context) { m2Called = true },
		nil,
	)

	pipeline := state.Action(func(ctx context.Context) {
		actionCalled = true
	})

	ctx := context.Background()
	ctx = transform.WithTransform(ctx, m1)
	ctx = transform.WithTransform(ctx, m2)

	transform.RunWithTransforms(ctx, pipeline)

	// Verify all middleware were applied
	if !m1Called {
		t.Error("middleware m1 was not called")
	}
	if !m2Called {
		t.Error("middleware m2 was not called")
	}
	if !actionCalled {
		t.Error("pipeline action was not called")
	}
}
