// Package workflows holds the Temporal workflow and activity definitions.
//
// M1 ships a trivial hello-world workflow proving the stack end to end; the
// CompanyResearch workflow replaces it in M2.
package workflows

import (
	"context"
	"errors"

	"go.temporal.io/sdk/workflow"
)

// TaskQueue is the task queue both the worker and the API client use.
const TaskQueue = "techcheck"

var errNotImplemented = errors.New("not implemented")

// Hello is the M1 placeholder workflow: it runs the SayHello activity and
// returns its greeting.
func Hello(ctx workflow.Context, name string) (string, error) {
	return "", errNotImplemented
}

// SayHello is the M1 placeholder activity.
func SayHello(ctx context.Context, name string) (string, error) {
	return "", errNotImplemented
}
