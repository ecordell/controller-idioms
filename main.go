package main

import (
	"context"
	"fmt"

	"github.com/authzed/controller-idioms/state"
)

func main() {
	ctx := context.Background()

	// Test the new naming scheme: Handler, NewStep, Step()
	pipeline := state.Sequence(
		state.Step(func(ctx context.Context) context.Context {
			fmt.Println("Step 1: Setting user")
			return context.WithValue(ctx, "user", "alice")
		}),
		state.Step(func(ctx context.Context) context.Context {
			user := ctx.Value("user").(string)
			fmt.Printf("Step 2: Processing user %s\n", user)
			return context.WithValue(ctx, "processed", true)
		}),
		state.Step(func(ctx context.Context) context.Context {
			processed := ctx.Value("processed").(bool)
			if processed {
				fmt.Println("Step 3: User processing completed!")
			}
			return ctx
		}),
	)

	fmt.Println("Running pipeline with new naming:")
	fmt.Println("- Handler: execution interface with Handle() method")
	fmt.Println("- NewHandler: the constructor type")
	fmt.Println("- Step(): the lifting function")
	fmt.Println()

	state.Run(ctx, pipeline)

	fmt.Println()
	fmt.Println("Context threading works perfectly!")
}
