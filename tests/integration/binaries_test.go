//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"

	"github.com/campoy/techcheck/internal/research"
)

// TestBinaries exercises the compiled worker and api binaries end to end,
// not in-process stand-ins: build both, run them against the Compose stack,
// start a research run over HTTP, and decode the resulting CompanyBrief.
// The worker runs with TECHCHECK_FAKE_PROVIDERS=1 so no API keys or money
// are involved (hermetic CI); the live suite covers the real providers.
func TestBinaries(t *testing.T) {
	bin := t.TempDir()
	briefs := t.TempDir()

	build := exec.CommandContext(t.Context(), "go", "build",
		"-o", bin, "../../worker", "../../api")
	build.Dir = "."
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build: %s", out)

	const apiAddr = "localhost:8091"

	worker := exec.CommandContext(t.Context(), filepath.Join(bin, "worker"))
	worker.Env = append(worker.Environ(),
		"TECHCHECK_FAKE_PROVIDERS=1",
		"TECHCHECK_BRIEFS_DIR="+briefs,
	)
	require.NoError(t, worker.Start())
	t.Cleanup(func() { _ = worker.Process.Kill() })

	api := exec.CommandContext(t.Context(), filepath.Join(bin, "api"))
	api.Env = append(api.Environ(), "TECHCHECK_API_ADDR="+apiAddr)
	require.NoError(t, api.Start())
	t.Cleanup(func() { _ = api.Process.Kill() })

	// Wait for the API to come up.
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
			"http://"+apiAddr+"/healthz", nil)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode == http.StatusOK
	}, 15*time.Second, 200*time.Millisecond, "api binary never became healthy")

	// Start a run through the real binary.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"http://"+apiAddr+"/companies/smoke-test-co/runs", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var body struct {
		WorkflowID string `json:"workflow_id"`
		RunID      string `json:"run_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "research-smoke-test-co", body.WorkflowID,
		"workflow IDs derive from the normalized company name (FR-1.2)")

	// The run must complete on the worker binary and produce a brief.
	c, err := client.Dial(client.Options{HostPort: temporalHostPort()})
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := contextWithTimeout(t, 60*time.Second)
	defer cancel()

	var brief research.CompanyBrief
	require.NoError(t, c.GetWorkflow(ctx, body.WorkflowID, body.RunID).Get(ctx, &brief))
	require.Equal(t, "smoke-test-co", brief.Company)
	require.NoError(t, brief.Validate())
}
