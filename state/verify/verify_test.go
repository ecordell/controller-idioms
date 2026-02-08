package verify_test

import (
	"context"
	"testing"

	"github.com/authzed/controller-idioms/state"
	"github.com/authzed/controller-idioms/state/verify"
)

func TestVerifyNoPanic_Success(t *testing.T) {
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	err := verify.VerifyNoPanic(pipeline)
	if err != nil {
		t.Errorf("expected no panic, got error: %v", err)
	}
}

func TestVerifyNoPanic_DetectsPanic(t *testing.T) {
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {
			panic("test panic")
		}),
	)

	err := verify.VerifyNoPanic(pipeline)
	if err == nil {
		t.Error("expected panic to be detected, got nil error")
	}
}
