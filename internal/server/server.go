// Package server implements the REST/JSON API in front of Temporal.
package server

import (
	"context"
	"net/http"

	"go.temporal.io/sdk/client"
)

// WorkflowStarter is the slice of the Temporal client the server needs, kept
// narrow so tests can fake it.
type WorkflowStarter interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow any, args ...any) (client.WorkflowRun, error)
}

// New returns the API handler: GET /healthz and
// POST /companies/{name}/runs, which starts a workflow on the techcheck
// task queue and replies 202 with the workflow and run IDs.
func New(temporal WorkflowStarter) http.Handler {
	return http.NewServeMux() // not implemented
}
