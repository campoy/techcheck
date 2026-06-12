// Package workflows holds the Temporal workflow and activity definitions.
//
// M1 ships a trivial hello-world workflow proving the stack end to end; the
// CompanyResearch workflow replaces it in M2.
package workflows

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// TaskQueue is the task queue both the worker and the API client use.
const TaskQueue = "techcheck"

// Hello is the M1 placeholder workflow: it runs the SayHello activity and
// returns its greeting.
func Hello(ctx workflow.Context, name string) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	var greeting string
	if err := workflow.ExecuteActivity(ctx, SayHello, name).Get(ctx, &greeting); err != nil {
		return "", err
	}
	return greeting, nil
}

// SayHello is the M1 placeholder activity.
func SayHello(ctx context.Context, name string) (string, error) {
	return fmt.Sprintf("Hello, %s!", name), nil
}
