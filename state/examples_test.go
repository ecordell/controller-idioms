package state

import (
	"context"
	"fmt"
)

// Example showing how the branching example from handler package would look in state package

func Example_branchingComparison() {
	ctx := context.Background()

	// This is equivalent to the complex handler branching example:
	// hasSecretHandler := chain(c.ensureMetadata, c.ensureScopedDatabaseCreds(...))
	// directSecretHandler := chain(c.adoptDBRootSecret, c.removeMissingSecretCondition, hasSecretHandler)
	// mainHandler := chain(c.setFinalizer, c.safeDelete, c.checkPause, c.validateInstance(hasSecretHandler, directSecretHandler))

	// Define reusable stage builders
	ensureMetadata := Action(func(ctx context.Context) {
		fmt.Println("ensuring metadata")
	})

	ensureScopedDatabaseCreds := Action(func(ctx context.Context) {
		fmt.Println("ensuring scoped database credentials")
	})

	createLogicalDatabase := Action(func(ctx context.Context) {
		fmt.Println("creating logical database")
	})

	adoptDBRootSecret := Action(func(ctx context.Context) {
		fmt.Println("adopting DB root secret")
	})

	removeMissingSecretCondition := Action(func(ctx context.Context) {
		fmt.Println("removing missing secret condition")
	})

	// hasSecretChain - runs when secret is found via database instance
	hasSecretChain := Sequence(
		ensureMetadata,
		Sequence(
			ensureScopedDatabaseCreds,
			createLogicalDatabase,
		),
	)

	// directSecretChain - runs when database instance not found (old style)
	directSecretChain := Sequence(
		adoptDBRootSecret,
		removeMissingSecretCondition,
		hasSecretChain, // Reuse the hasSecret chain
	)

	// validateInstance - decides between the two branches
	validateInstance := Decision(
		func(ctx context.Context) bool {
			// In real code, this would check if database instance exists
			fmt.Println("validating instance")
			return true // Has database instance
		},
		hasSecretChain,    // True branch
		directSecretChain, // False branch
	)

	// Main controller pipeline
	mainHandler := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("setting finalizer")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("safe delete check")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("checking pause")
		}),
		validateInstance,
	)

	Run(ctx, mainHandler)

	// Output:
	// setting finalizer
	// safe delete check
	// checking pause
	// validating instance
	// ensuring metadata
	// ensuring scoped database credentials
	// creating logical database
}

// Example showing monadic composition patterns

func Example_monadicPatterns() {
	ctx := context.WithValue(context.Background(), "config", "production")

	// Using Map to transform context
	pipeline := Sequence(
		Map(
			func(ctx context.Context) context.Context {
				config := ctx.Value("config").(string)
				return context.WithValue(ctx, "validated-config", config+"-validated")
			},
			Action(func(ctx context.Context) {
				config := ctx.Value("validated-config").(string)
				fmt.Printf("using config: %s\n", config)
			}),
		),
		// Using Bind for dependent composition
		Bind(
			Action(func(ctx context.Context) {
				fmt.Println("first operation")
			}),
			func() NewStep {
				return Action(func(ctx context.Context) {
					fmt.Println("dependent operation")
				})
			},
		),
	)

	Run(ctx, pipeline)

	// Output:
	// using config: production-validated
	// first operation
	// dependent operation
}

// Example showing conditional execution based on context

func Example_conditionalExecution() {
	ctx := context.WithValue(context.Background(), "shouldProcess", false)

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("starting process")
		}),
		Decision(
			func(ctx context.Context) bool {
				return ctx.Value("shouldProcess").(bool)
			},
			Action(func(ctx context.Context) {
				fmt.Println("processing enabled")
			}),
			Action(func(ctx context.Context) {
				fmt.Println("processing disabled")
			}),
		),
		Action(func(ctx context.Context) {
			fmt.Println("cleanup")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// starting process
	// processing disabled
	// cleanup
}

// Example showing complex conditional logic simplified with Enum

func Example_complexConditionals() {
	ctx := context.WithValue(context.Background(), "user-role", "admin")

	// Multi-way branching using Enum - much cleaner than nested decisions
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("authentication")
		}),
		Enum(
			func(ctx context.Context) string {
				return ctx.Value("user-role").(string)
			},
			map[string]NewStep{
				"admin": Action(func(ctx context.Context) {
					fmt.Println("admin workflow")
				}),
				"user": Action(func(ctx context.Context) {
					fmt.Println("user workflow")
				}),
				"moderator": Action(func(ctx context.Context) {
					fmt.Println("moderator workflow")
				}),
			},
			Action(func(ctx context.Context) {
				fmt.Println("guest workflow")
			}),
		),
		Action(func(ctx context.Context) {
			fmt.Println("logging user action")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// authentication
	// admin workflow
	// logging user action
}

// Example showing reusable stage composition

func Example_reusableStages() {
	ctx := context.Background()

	// Define reusable stages
	logStart := func(name string) NewStep {
		return Action(func(ctx context.Context) {
			fmt.Printf("starting %s\n", name)
		})
	}

	logEnd := func(name string) NewStep {
		return Action(func(ctx context.Context) {
			fmt.Printf("completed %s\n", name)
		})
	}

	// Wrap a stage with logging
	withLogging := func(name string, stage NewStep) NewStep {
		return Sequence(
			logStart(name),
			stage,
			logEnd(name),
		)
	}

	// Use the reusable pattern
	pipeline := Sequence(
		withLogging("validation", Action(func(ctx context.Context) {
			fmt.Println("validating input")
		})),
		withLogging("processing", Action(func(ctx context.Context) {
			fmt.Println("processing work")
		})),
		withLogging("cleanup", Action(func(ctx context.Context) {
			fmt.Println("cleaning up")
		})),
	)

	Run(ctx, pipeline)

	// Output:
	// starting validation
	// validating input
	// completed validation
	// starting processing
	// processing work
	// completed processing
	// starting cleanup
	// cleaning up
	// completed cleanup
}

// Example showing how this replaces the Builder pattern complexity

func Example_builderReplacement() {
	ctx := context.Background()

	// In the old handler system, you needed:
	// 1. Builder functions
	// 2. Chain() to compose builders
	// 3. .Handler(id) to instantiate
	// 4. Complex ID management for branching

	// In the state system, it's much simpler:
	// Just compose NewStep functions directly

	// Old way (conceptually):
	// validationBuilder := func(next Handler) Handler { ... }
	// processingBuilder := func(next Handler) Handler { ... }
	// pipeline := Chain(validationBuilder, processingBuilder).Handler("myPipeline")

	// New way:
	validation := Action(func(ctx context.Context) {
		fmt.Println("validation")
	})

	processing := Action(func(ctx context.Context) {
		fmt.Println("processing")
	})

	pipeline := Sequence(validation, processing)

	Run(ctx, pipeline)

	// Output:
	// validation
	// processing
}

// Example showing Enum operator for multi-way branching

func Example_enumBranching() {
	ctx := context.WithValue(context.Background(), "resource-type", "deployment")

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("processing resource")
		}),
		Enum(
			func(ctx context.Context) string {
				return ctx.Value("resource-type").(string)
			},
			map[string]NewStep{
				"deployment": Sequence(
					Action(func(ctx context.Context) {
						fmt.Println("validating deployment spec")
					}),
					Action(func(ctx context.Context) {
						fmt.Println("creating deployment")
					}),
				),
				"service": Sequence(
					Action(func(ctx context.Context) {
						fmt.Println("validating service spec")
					}),
					Action(func(ctx context.Context) {
						fmt.Println("creating service")
					}),
				),
				"configmap": Action(func(ctx context.Context) {
					fmt.Println("creating configmap")
				}),
			},
			Action(func(ctx context.Context) {
				fmt.Println("unsupported resource type")
			}),
		),
		Action(func(ctx context.Context) {
			fmt.Println("resource processing complete")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// processing resource
	// validating deployment spec
	// creating deployment
	// resource processing complete
}

// Example showing Switch operator for status-based workflows

func Example_switchWorkflow() {
	ctx := context.WithValue(context.Background(), "phase", "pending")

	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("checking resource phase")
		}),
		Switch(
			func(ctx context.Context) string {
				return ctx.Value("phase").(string)
			},
			map[string]NewStep{
				"pending": Sequence(
					Action(func(ctx context.Context) {
						fmt.Println("initializing resources")
					}),
					Action(func(ctx context.Context) {
						fmt.Println("setting up dependencies")
					}),
				),
				"running": Action(func(ctx context.Context) {
					fmt.Println("monitoring running state")
				}),
				"failed": Sequence(
					Action(func(ctx context.Context) {
						fmt.Println("analyzing failure")
					}),
					Action(func(ctx context.Context) {
						fmt.Println("attempting recovery")
					}),
				),
				"completed": Action(func(ctx context.Context) {
					fmt.Println("cleaning up completed resources")
				}),
			},
			Action(func(ctx context.Context) {
				fmt.Println("unknown phase - logging for investigation")
			}),
		),
	)

	Run(ctx, pipeline)

	// Output:
	// checking resource phase
	// initializing resources
	// setting up dependencies
}

// Example showing complex controller logic with Enum

func Example_controllerWithEnum() {
	ctx := context.WithValue(context.Background(), "operation", "reconcile")

	// Define reusable stages
	setFinalizer := Action(func(ctx context.Context) {
		fmt.Println("setting finalizer")
	})

	validateSpec := Action(func(ctx context.Context) {
		fmt.Println("validating spec")
	})

	createResources := Action(func(ctx context.Context) {
		fmt.Println("creating resources")
	})

	updateResources := Action(func(ctx context.Context) {
		fmt.Println("updating resources")
	})

	deleteResources := Action(func(ctx context.Context) {
		fmt.Println("deleting resources")
	})

	removeFinalizer := Action(func(ctx context.Context) {
		fmt.Println("removing finalizer")
	})

	// Main controller pipeline using Enum for operation dispatch
	pipeline := Sequence(
		Action(func(ctx context.Context) {
			fmt.Println("starting controller operation")
		}),
		Enum(
			func(ctx context.Context) string {
				return ctx.Value("operation").(string)
			},
			map[string]NewStep{
				"reconcile": Sequence(
					setFinalizer,
					validateSpec,
					Decision(
						func(ctx context.Context) bool {
							// Check if resources exist
							return false // Assume they don't exist
						},
						updateResources,
						createResources,
					),
				),
				"delete": Sequence(
					deleteResources,
					removeFinalizer,
				),
				"validate": validateSpec,
			},
			Action(func(ctx context.Context) {
				fmt.Println("unsupported operation")
			}),
		),
		Action(func(ctx context.Context) {
			fmt.Println("operation completed")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// starting controller operation
	// setting finalizer
	// validating spec
	// creating resources
	// operation completed
}

// Example showing stage reuse - replicating the old handler wrapping pattern

func Example_stageReuse() {
	ctx := context.WithValue(context.Background(), "validations", []string{"policy1", "policy2"})

	// Define a reusable validation stage
	ensureValidatingAdmissionPolicy := Action(func(ctx context.Context) {
		fmt.Println("ensuring validating admission policy")
	})

	// Replicate the old handler pattern:
	// - Skip if no validations are specified
	// - Otherwise do validation work and check for cancellation
	// - Then continue to next stage
	pipeline := Sequence(
		func(next Step) Step {
			return StepFunc(func(ctx context.Context) Step {
				validations := ctx.Value("validations").([]string)

				// Skip if no validations are specified
				if len(validations) == 0 {
					if next != nil {
						return next.Run(ctx)
					}
					return nil
				}

				// Ensure ValidatingAdmissionPolicy
				ensureValidatingAdmissionPolicy(nil).Run(ctx)

				// catch errors that happened inside the component handler
				if ctx.Err() != nil {
					return nil
				}

				if next != nil {
					return next.Run(ctx)
				}
				return nil
			})
		},
		Action(func(ctx context.Context) {
			fmt.Println("continuing with main logic")
		}),
		Action(func(ctx context.Context) {
			fmt.Println("cleanup after validation")
		}),
	)

	Run(ctx, pipeline)

	// Output:
	// ensuring validating admission policy
	// continuing with main logic
	// cleanup after validation
}

// Example showing CallAndContinueIf pattern

func Example_callAndContinueIf() {
	ctx := context.WithValue(context.Background(), "hasErrors", false)

	validationStage := Action(func(ctx context.Context) {
		fmt.Println("running validation")
	})

	successStage := Action(func(ctx context.Context) {
		fmt.Println("validation succeeded - continuing")
	})

	errorStage := Action(func(ctx context.Context) {
		fmt.Println("validation failed - stopping")
	})

	pipeline := CallAndContinueIf(
		validationStage,
		func(ctx context.Context) bool {
			// Check if validation succeeded (no errors in context)
			return !ctx.Value("hasErrors").(bool)
		},
		successStage, // Continue here if condition is true
		errorStage,   // Go here if condition is false
	)

	Run(ctx, pipeline)

	// Output:
	// running validation
	// validation succeeded - continuing
}

// Example showing CallAndCheck for complex result handling

func Example_callAndCheck() {
	ctx := context.Background()

	// Step that might succeed or fail
	riskyStagetage := Action(func(ctx context.Context) {
		fmt.Println("executing risky operation")
		// Simulate some work that might set error state
	})

	pipeline := CallAndCheck(
		riskyStagetage,
		func(ctx context.Context, result Step) Step {
			// Inspect context or result to decide what to do next
			if ctx.Err() != nil {
				fmt.Println("operation failed - running recovery")
				return Action(func(ctx context.Context) {
					fmt.Println("recovery completed")
				}).Step()
			}

			fmt.Println("operation succeeded - running cleanup")
			return Action(func(ctx context.Context) {
				fmt.Println("cleanup completed")
			}).Step()
		},
	)

	Run(ctx, pipeline)

	// Output:
	// executing risky operation
	// operation succeeded - running cleanup
	// cleanup completed
}
