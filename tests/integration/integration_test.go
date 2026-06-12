//go:build integration

// Package integration holds smoke tests that require the Docker Compose
// stack to be running (make up). They validate M1: the stack comes up, the
// API reaches Temporal, and a workflow round-trips with inspectable history.
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/server"
	"github.com/campoy/techcheck/internal/workflows"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func pgDSN() string {
	return envOr("TECHCHECK_TEST_PG_DSN",
		"postgres://postgres:postgres@localhost:5432/research?sslmode=disable")
}

func temporalHostPort() string {
	return envOr("TECHCHECK_TEST_TEMPORAL", "localhost:7233")
}

func uiURL() string {
	return envOr("TECHCHECK_TEST_UI", "http://localhost:8080")
}

// TestPostgres: the research database accepts connections and the pgvector
// extension is installable.
func TestPostgres(t *testing.T) {
	db, err := sql.Open("pgx", pgDSN())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, db.PingContext(ctx))
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	require.NoError(t, err, "pgvector must be available in the research database")
}

// TestTemporalHealth: the Temporal frontend answers a health check.
func TestTemporalHealth(t *testing.T) {
	c, err := client.Dial(client.Options{HostPort: temporalHostPort()})
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = c.CheckHealth(ctx, &client.CheckHealthRequest{})
	require.NoError(t, err)
}

// TestWebUI: the Temporal Web UI serves HTTP.
func TestWebUI(t *testing.T) {
	httpc := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, uiURL(), nil)
	require.NoError(t, err)
	resp, err := httpc.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestEndToEnd: a run started through the API completes on a worker and its
// event history is retrievable through the Temporal client (FR-1.4; FR-9.1
// partial). As of M2 the API starts CompanyResearch (FR-1.1), so the worker
// registers it with fake providers to stay hermetic; the M1 greeting
// assertion is superseded by decoding a CompanyBrief.
func TestEndToEnd(t *testing.T) {
	c, err := client.Dial(client.Options{HostPort: temporalHostPort()})
	require.NoError(t, err)
	defer c.Close()

	// In-process worker against the real server, same registrations the
	// worker binary uses, with hermetic providers.
	w := worker.New(c, workflows.TaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.Hello)
	w.RegisterActivity(workflows.SayHello)
	w.RegisterWorkflow(research.CompanyResearch)
	w.RegisterActivity(&research.Activities{
		LLM:       fakeLLM{},
		Searcher:  fakeSearcher{},
		BriefsDir: t.TempDir(),
	})
	require.NoError(t, w.Start())
	defer w.Stop()

	api := httptest.NewServer(server.New(c))
	defer api.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		api.URL+"/companies/acme/runs", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var body struct {
		WorkflowID string `json:"workflow_id"`
		RunID      string `json:"run_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body.WorkflowID)
	require.NotEmpty(t, body.RunID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var brief research.CompanyBrief
	require.NoError(t, c.GetWorkflow(ctx, body.WorkflowID, body.RunID).Get(ctx, &brief))
	require.Equal(t, "acme", brief.Company)
	require.NoError(t, brief.Validate())

	// The run's event history must be inspectable after completion.
	iter := c.GetWorkflowHistory(ctx, body.WorkflowID, body.RunID, false,
		enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	events := 0
	for iter.HasNext() {
		_, err := iter.Next()
		require.NoError(t, err)
		events++
	}
	require.Positive(t, events, "completed run must have retrievable history")
}
