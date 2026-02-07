package state

import (
	"context"
	"testing"
)

func TestDoWithStageAsWrapper(t *testing.T) {
	// Do as a stage wrapper - transforms context before executing wrapped stage
	Do := func(transform func(context.Context) context.Context, stage NewStage) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				// Transform context first
				newCtx := transform(ctx)
				// Then execute the wrapped stage with transformed context
				wrappedStage := stage(next)
				if wrappedStage != nil {
					return wrappedStage.Next(newCtx)
				}
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	ctx := context.Background()
	var results []string

	// Base stage that reads from context
	readUser := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			if user := ctx.Value("user"); user != nil {
				results = append(results, "user-is-"+user.(string))
			} else {
				results = append(results, "no-user")
			}
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	// Wrap the stage with context transformation
	pipeline := Sequence(
		Do(func(ctx context.Context) context.Context {
			return context.WithValue(ctx, "user", "alice")
		}, readUser),
		readUser, // Should still see alice from previous stage
	)

	Run(ctx, pipeline)

	expected := []string{
		"user-is-alice",
		"user-is-alice",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}
}

func TestDoAsStageTransformer(t *testing.T) {
	// Do returns a function that transforms stages
	Do := func(transform func(context.Context) context.Context) func(NewStage) NewStage {
		return func(stage NewStage) NewStage {
			return func(next Stage) Stage {
				return StageFunc(func(ctx context.Context) Stage {
					newCtx := transform(ctx)
					wrappedStage := stage(next)
					if wrappedStage != nil {
						return wrappedStage.Next(newCtx)
					}
					if next != nil {
						return next.Next(newCtx)
					}
					return nil
				})
			}
		}
	}

	ctx := context.Background()
	var results []string

	// Base stages
	readUser := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			if user := ctx.Value("user"); user != nil {
				results = append(results, "reading-user-"+user.(string))
			}
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	readRole := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			if role := ctx.Value("role"); role != nil {
				results = append(results, "reading-role-"+role.(string))
			}
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	// Create transformers
	addUser := Do(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "user", "bob")
	})

	addRole := Do(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "role", "admin")
	})

	// Compose stages with transformers
	pipeline := Sequence(
		addUser(readUser),
		addRole(readRole),
		readUser, // Should still see bob
		readRole, // Should still see admin
	)

	Run(ctx, pipeline)

	expected := []string{
		"reading-user-bob",
		"reading-role-admin",
		"reading-user-bob",
		"reading-role-admin",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}
}

func TestDoAsStageComposition(t *testing.T) {
	// Do composes two stages in sequence
	Do := func(preStage NewStage, postStage NewStage) NewStage {
		return func(next Stage) Stage {
			// Chain: preStage -> postStage -> next
			return preStage(postStage(next))
		}
	}

	ctx := context.Background()
	var results []string

	setUser := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			results = append(results, "setting-user")
			newCtx := context.WithValue(ctx, "user", "charlie")
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	setRole := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			results = append(results, "setting-role")
			newCtx := context.WithValue(ctx, "role", "user")
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	logContext := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			user := ctx.Value("user")
			role := ctx.Value("role")
			results = append(results, "context-"+user.(string)+"-"+role.(string))
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	// Compose stages
	pipeline := Sequence(
		Do(setUser, setRole),
		logContext,
	)

	Run(ctx, pipeline)

	expected := []string{
		"setting-user",
		"setting-role",
		"context-charlie-user",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}
}

func TestDoWithBeforeAfter(t *testing.T) {
	// Do wraps a stage with before and after behavior
	Do := func(before func(context.Context) context.Context, stage NewStage, after func(context.Context) context.Context) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				// Before
				beforeCtx := before(ctx)

				// Execute wrapped stage with a continuation that applies "after"
				continuation := StageFunc(func(ctx context.Context) Stage {
					// After (before continuing to next)
					afterCtx := after(ctx)
					if next != nil {
						return next.Next(afterCtx)
					}
					return nil
				})
				wrappedStage := stage(continuation)

				if wrappedStage != nil {
					return wrappedStage.Next(beforeCtx)
				}
				if next != nil {
					return next.Next(beforeCtx)
				}
				return nil
			})
		}
	}

	ctx := context.Background()
	var results []string

	coreLogic := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			user := ctx.Value("user").(string)
			results = append(results, "processing-"+user)
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	pipeline := Sequence(
		Do(
			func(ctx context.Context) context.Context {
				results = append(results, "before")
				return context.WithValue(ctx, "user", "diana")
			},
			coreLogic,
			func(ctx context.Context) context.Context {
				results = append(results, "after")
				return context.WithValue(ctx, "completed", true)
			},
		),
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				if completed := ctx.Value("completed"); completed != nil {
					results = append(results, "cleanup-completed")
				}
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		},
	)

	Run(ctx, pipeline)

	expected := []string{
		"before",
		"processing-diana",
		"after",
		"cleanup-completed",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d: %v", len(expected), len(results), results)
	}

	for i, exp := range expected {
		if results[i] != exp {
			t.Errorf("Expected results[%d] = %q, got %q", i, exp, results[i])
		}
	}
}

func TestDoReadabilityComparison(t *testing.T) {
	_ = context.Background()

	// Base stage for testing
	baseStage := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// does some work
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	// 1. Original inline approach
	_ = Sequence(
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				newCtx := context.WithValue(ctx, "user", "alice")
				wrappedStage := baseStage(next)
				if wrappedStage != nil {
					return wrappedStage.Next(newCtx)
				}
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		},
	)

	// 2. Do as wrapper
	DoWrapper := func(transform func(context.Context) context.Context, stage NewStage) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				newCtx := transform(ctx)
				wrappedStage := stage(next)
				if wrappedStage != nil {
					return wrappedStage.Next(newCtx)
				}
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	_ = Sequence(
		DoWrapper(func(ctx context.Context) context.Context {
			return context.WithValue(ctx, "user", "alice")
		}, baseStage),
	)

	// 3. Do as transformer
	DoTransformer := func(transform func(context.Context) context.Context) func(NewStage) NewStage {
		return func(stage NewStage) NewStage {
			return func(next Stage) Stage {
				return StageFunc(func(ctx context.Context) Stage {
					newCtx := transform(ctx)
					wrappedStage := stage(next)
					if wrappedStage != nil {
						return wrappedStage.Next(newCtx)
					}
					if next != nil {
						return next.Next(newCtx)
					}
					return nil
				})
			}
		}
	}

	addUser := DoTransformer(func(ctx context.Context) context.Context {
		return context.WithValue(ctx, "user", "alice")
	})

	_ = Sequence(
		addUser(baseStage),
	)

	t.Log("All Do variations compiled successfully")
	t.Log("DoWrapper: Do(transform, stage)")
	t.Log("DoTransformer: addUser := Do(transform); addUser(stage)")
}
