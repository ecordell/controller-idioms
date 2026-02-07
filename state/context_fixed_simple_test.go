package state

import (
	"context"
	"testing"
)

func TestContextThreadingNowWorksWithAction(t *testing.T) {
	ctx := context.Background()

	// Track what values we see in each stage
	var stage1Value, stage2Value, stage3Value string

	// Create a helper that adds a value to context
	addToContext := func(key, value string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				// Add value to context
				newCtx := context.WithValue(ctx, key, value)

				// Continue to next stage with modified context
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	// Create stages that read from context
	stage1 := Action(func(ctx context.Context) {
		if val := ctx.Value("key"); val != nil {
			stage1Value = val.(string)
		}
	})

	stage2 := Action(func(ctx context.Context) {
		if val := ctx.Value("key"); val != nil {
			stage2Value = val.(string)
		}
	})

	stage3 := Action(func(ctx context.Context) {
		if val := ctx.Value("key"); val != nil {
			stage3Value = val.(string)
		}
	})

	// Now we can thread context through the pipeline
	pipeline := Sequence(
		stage1,                        // Should see empty
		addToContext("key", "value1"), // Add first value
		stage2,                        // Should see "value1"
		addToContext("key", "value2"), // Change to second value
		stage3,                        // Should see "value2"
	)

	Run(ctx, pipeline)

	t.Logf("Stage1 saw: %q", stage1Value)
	t.Logf("Stage2 saw: %q", stage2Value)
	t.Logf("Stage3 saw: %q", stage3Value)

	// Context threading now works correctly
	if stage1Value != "" {
		t.Errorf("Expected stage1 to see empty context, got %q", stage1Value)
	}
	if stage2Value != "value1" {
		t.Errorf("Expected stage2 to see 'value1', got %q", stage2Value)
	}
	if stage3Value != "value2" {
		t.Errorf("Expected stage3 to see 'value2', got %q", stage3Value)
	}
}

func TestDecisionWithContextThreading(t *testing.T) {
	ctx := context.WithValue(context.Background(), "condition", "initial")

	var predicateValue string
	var branchValue string

	// Helper to modify context
	setCondition := func(value string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				newCtx := context.WithValue(ctx, "condition", value)
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	pipeline := Sequence(
		setCondition("modified"), // Change condition
		Decision(
			func(ctx context.Context) bool {
				val := ctx.Value("condition").(string)
				predicateValue = val
				return val == "modified"
			},
			Action(func(ctx context.Context) {
				branchValue = "true-branch"
			}),
			Action(func(ctx context.Context) {
				branchValue = "false-branch"
			}),
		),
	)

	Run(ctx, pipeline)

	// The decision should now see the modified context
	if predicateValue != "modified" {
		t.Errorf("Expected predicate to see 'modified', got %q", predicateValue)
	}
	if branchValue != "true-branch" {
		t.Errorf("Expected true branch to execute, got %q", branchValue)
	}
}

func TestEnumWithContextThreading(t *testing.T) {
	ctx := context.WithValue(context.Background(), "type", "initial")

	var enumValue string
	var branchValue string

	// Helper to change the resource type
	setResourceType := func(resourceType string) NewStage {
		return func(next Stage) Stage {
			return StageFunc(func(ctx context.Context) Stage {
				newCtx := context.WithValue(ctx, "type", resourceType)
				if next != nil {
					return next.Next(newCtx)
				}
				return nil
			})
		}
	}

	pipeline := Sequence(
		setResourceType("deployment"), // Change type to deployment
		Enum(
			func(ctx context.Context) string {
				val := ctx.Value("type").(string)
				enumValue = val
				return val
			},
			map[string]NewStage{
				"deployment": Action(func(ctx context.Context) {
					branchValue = "deployment-handler"
				}),
				"service": Action(func(ctx context.Context) {
					branchValue = "service-handler"
				}),
			},
			Action(func(ctx context.Context) {
				branchValue = "default-handler"
			}),
		),
	)

	Run(ctx, pipeline)

	// The enum should now see the modified context
	if enumValue != "deployment" {
		t.Errorf("Expected enum to see 'deployment', got %q", enumValue)
	}
	if branchValue != "deployment-handler" {
		t.Errorf("Expected deployment branch to execute, got %q", branchValue)
	}
}
