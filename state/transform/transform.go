package transform

import "github.com/authzed/controller-idioms/state"

// WrapStep wraps a Step with a recursive middleware wrapper.
// This is an internal utility for implementing recursive middleware.
// If step is nil, returns nil.
func WrapStep(step state.Step, wrapper func(state.NewStep) state.NewStep) state.Step {
	if step == nil {
		return nil
	}
	// Convert Step to NewStep, apply wrapper, then convert back
	return wrapper(func(next state.Step) state.Step {
		return step
	})(nil)
}
