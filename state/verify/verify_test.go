package verify_test

import (
	"context"
	"testing"
	"time"

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

func TestVerifyTerminates_Success(t *testing.T) {
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	err := verify.VerifyTerminates(pipeline, 1*time.Second)
	if err != nil {
		t.Errorf("expected pipeline to terminate, got error: %v", err)
	}
}

func TestVerifyTerminates_Timeout(t *testing.T) {
	pipeline := state.Action(func(ctx context.Context) {
		time.Sleep(100 * time.Millisecond)
	})

	err := verify.VerifyTerminates(pipeline, 10*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestVerifyProgress_Success(t *testing.T) {
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	err := verify.VerifyProgress(pipeline, 10)
	if err != nil {
		t.Errorf("expected progress verification to pass, got error: %v", err)
	}
}

func TestVerifyProgress_ExceedsMax(t *testing.T) {
	pipeline := state.Sequence(
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
		state.Action(func(ctx context.Context) {}),
	)

	err := verify.VerifyProgress(pipeline, 2)
	if err == nil {
		t.Error("expected progress verification to fail, got nil")
	}
}
