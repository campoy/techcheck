package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"

	"github.com/campoy/techcheck/internal/server"
	"github.com/campoy/techcheck/internal/workflows"
)

// fakeStarter records the workflow start request and returns canned IDs.
type fakeStarter struct {
	options client.StartWorkflowOptions
	called  bool
}

func (f *fakeStarter) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow any, args ...any) (client.WorkflowRun, error) {
	f.called = true
	f.options = options
	return fakeRun{}, nil
}

type fakeRun struct{ client.WorkflowRun }

func (fakeRun) GetID() string    { return "research-acme" }
func (fakeRun) GetRunID() string { return "run-1" }

func TestHealthz(t *testing.T) {
	h := server.New(&fakeStarter{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestStartRun(t *testing.T) {
	starter := &fakeStarter{}
	h := server.New(starter)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/companies/acme/runs", nil))

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.True(t, starter.called, "handler must start a workflow")
	require.Equal(t, workflows.TaskQueue, starter.options.TaskQueue)

	var body struct {
		WorkflowID string `json:"workflow_id"`
		RunID      string `json:"run_id"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "research-acme", body.WorkflowID)
	require.Equal(t, "run-1", body.RunID)
}

func TestStartRunRequiresCompany(t *testing.T) {
	starter := &fakeStarter{}
	h := server.New(starter)

	for _, path := range []string{"/companies//runs", "/companies/%20/runs"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
		require.True(t, rec.Code >= 400 && rec.Code < 500,
			"POST %s should be a client error, got %d", path, rec.Code)
		require.False(t, starter.called, "no workflow should start for %s", path)
	}
}

func TestStartRunMethodNotAllowed(t *testing.T) {
	h := server.New(&fakeStarter{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/companies/acme/runs", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
