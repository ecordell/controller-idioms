package state

import (
	"context"
	"testing"
)

func TestContextThreadingFixed(t *testing.T) {
	ctx := context.Background()

	// Track what values we see in each stage
	var stage1Value, stage2Value, stage3Value string

	stage1 := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// Should see empty value initially
			if val := ctx.Value("key"); val != nil {
				stage1Value = val.(string)
			}

			// Add value to context
			newCtx := context.WithValue(ctx, "key", "from-stage1")

			// Continue to next stage with modified context
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	stage2 := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// Should see "from-stage1"
			if val := ctx.Value("key"); val != nil {
				stage2Value = val.(string)
			}

			// Modify context again
			newCtx := context.WithValue(ctx, "key", "from-stage2")

			// Continue to next stage with modified context
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	stage3 := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// Should see "from-stage2"
			if val := ctx.Value("key"); val != nil {
				stage3Value = val.(string)
			}

			// Continue to next stage
			if next != nil {
				return next.Next(ctx)
			}
			return nil
		})
	}

	pipeline := Sequence(stage1, stage2, stage3)
	Run(ctx, pipeline)

	t.Logf("Stage1 saw: %q", stage1Value)
	t.Logf("Stage2 saw: %q", stage2Value)
	t.Logf("Stage3 saw: %q", stage3Value)

	// Now context threading should work correctly
	if stage1Value != "" {
		t.Errorf("Expected stage1 to see empty context, got %q", stage1Value)
	}
	if stage2Value != "from-stage1" {
		t.Errorf("Expected stage2 to see 'from-stage1', got %q", stage2Value)
	}
	if stage3Value != "from-stage2" {
		t.Errorf("Expected stage3 to see 'from-stage2', got %q", stage3Value)
	}
}

func TestContextThreadingWithAction(t *testing.T) {
	ctx := context.Background()

	var values []string

	// Create an Action that modifies context by wrapping it
	addValue := func(key, value string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				values = append(values, "adding-"+value)
				newCtx := context.WithValue(ctx, key, value)
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	// Create an Action that reads from context
	readValue := func(key string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				if val := ctx.Value(key); val != nil {
					values = append(values, "reading-"+val.(string))
				} else {
					values = append(values, "reading-empty")
				}
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		}
	}

	pipeline := Sequence(
		readValue("key"),          // Should see empty
		addValue("key", "first"),  // Add first value
		readValue("key"),          // Should see "first"
		addValue("key", "second"), // Override with second value
		readValue("key"),          // Should see "second"
	)

	Run(ctx, pipeline)

	expected := []string{
		"reading-empty",
		"adding-first",
		"reading-first",
		"adding-second",
		"reading-second",
	}

	if len(values) != len(expected) {
		t.Fatalf("Expected %d values, got %d: %v", len(expected), len(values), values)
	}

	for i, exp := range expected {
		if values[i] != exp {
			t.Errorf("Expected values[%d] = %q, got %q", i, exp, values[i])
		}
	}
}

func TestContextThreadingWithDecision(t *testing.T) {
	ctx := context.WithValue(context.Background(), "condition", "initial")

	var predicateValue string
	var branchValue string

	setupStage := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// Modify context to affect decision
			newCtx := context.WithValue(ctx, "condition", "modified")
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	decision := Decision(
		func(ctx context.Context) bool {
			// Should now see "modified" instead of "initial"
			val := ctx.Value("condition").(string)
			predicateValue = val
			return val == "modified"
		},
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				branchValue = "true-branch"
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		},
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				branchValue = "false-branch"
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		},
	)

	pipeline := Sequence(setupStage, decision)
	Run(ctx, pipeline)

	t.Logf("Decision predicate saw: %q", predicateValue)
	t.Logf("Branch executed: %q", branchValue)

	// Now context threading works: predicate sees "modified", true branch runs
	if predicateValue != "modified" {
		t.Errorf("Expected predicate to see 'modified', got %q", predicateValue)
	}
	if branchValue != "true-branch" {
		t.Errorf("Expected true branch to execute, got %q", branchValue)
	}
}

func TestContextThreadingWithEnum(t *testing.T) {
	ctx := context.WithValue(context.Background(), "type", "initial")

	var enumValue string
	var branchValue string

	setupStage := func(next Stage) Stage {
		return StageFunc(func(ctx context.Context) Stage {
			// Change the type to affect enum decision
			newCtx := context.WithValue(ctx, "type", "deployment")
			if next != nil {
				return next.Next(newCtx)
			}
			return nil
		})
	}

	enumStage := Enum(
		func(ctx context.Context) string {
			// Should now see "deployment" instead of "initial"
			val := ctx.Value("type").(string)
			enumValue = val
			return val
		},
		map[string]NewStage{
			"deployment": func(next Stage) Stage {
				return StageFunc(func(ctx context.Context) Stage {
					branchValue = "deployment-branch"
					if next != nil {
						return next.Next(ctx)
					}
					return nil
				})
			},
			"service": func(next Stage) Stage {
				return StageFunc(func(ctx context.Context) Stage {
					branchValue = "service-branch"
					if next != nil {
						return next.Next(ctx)
					}
					return nil
				})
			},
		},
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				branchValue = "default-branch"
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		},
	)

	pipeline := Sequence(setupStage, enumStage)
	Run(ctx, pipeline)

	t.Logf("Enum saw: %q", enumValue)
	t.Logf("Branch executed: %q", branchValue)

	// Now context threading works: enum sees "deployment", deployment branch runs
	if enumValue != "deployment" {
		t.Errorf("Expected enum to see 'deployment', got %q", enumValue)
	}
	if branchValue != "deployment-branch" {
		t.Errorf("Expected deployment branch to execute, got %q", branchValue)
	}
}

func TestContextThreadingWithMap(t *testing.T) {
	ctx := context.WithValue(context.Background(), "value", "original")

	var transformedValue string
	var finalValue string

	// Use Map to transform the context
	pipeline := Sequence(
		Map(
			func(ctx context.Context) context.Context {
				original := ctx.Value("value").(string)
				return context.WithValue(ctx, "value", original+"-transformed")
			},
			func(next Stage) Stage {
				return StageFunc(func(ctx context.Context) Stage {
					transformedValue = ctx.Value("value").(string)
					// Add another transformation
					newCtx := context.WithValue(ctx, "value", transformedValue+"-again")
					if next != nil {
						return next.Next(newCtx)
					}
					return nil
				})
			},
		),
		func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				finalValue = ctx.Value("value").(string)
				if next != nil {
					return next.Next(ctx)
				}
				return nil
			})
		},
	)

	Run(ctx, pipeline)

	t.Logf("Transformed value: %q", transformedValue)
	t.Logf("Final value: %q", finalValue)

	if transformedValue != "original-transformed" {
		t.Errorf("Expected transformed value to be 'original-transformed', got %q", transformedValue)
	}
	if finalValue != "original-transformed-again" {
		t.Errorf("Expected final value to be 'original-transformed-again', got %q", finalValue)
	}
}
