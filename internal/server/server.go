// Package server implements the REST/JSON API in front of Temporal.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"go.temporal.io/sdk/client"

	"github.com/campoy/techcheck/internal/corpus"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/workflows"
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
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})

	mux.HandleFunc("POST /companies/{name}/runs", func(w http.ResponseWriter, r *http.Request) {
		company := strings.TrimSpace(r.PathValue("name"))
		if company == "" {
			http.Error(w, "company name required", http.StatusBadRequest)
			return
		}

		run, err := temporal.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        "research-" + research.Normalize(company),
			TaskQueue: workflows.TaskQueue,
		}, research.CompanyResearch, research.ResearchInput{Company: company})
		if err != nil {
			http.Error(w, "starting workflow: "+err.Error(), http.StatusInternalServerError)
			return
		}
		respondStarted(w, run)
	})

	mux.HandleFunc("POST /corpus/ingest", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Dir string `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "decoding request: "+err.Error(), http.StatusBadRequest)
			return
		}

		run, err := temporal.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        "corpus-ingest",
			TaskQueue: workflows.TaskQueue,
		}, corpus.IngestCorpus, corpus.IngestInput{Dir: body.Dir})
		if err != nil {
			http.Error(w, "starting ingestion: "+err.Error(), http.StatusInternalServerError)
			return
		}
		respondStarted(w, run)
	})

	// ServeMux cleans paths like /companies//runs with a 307 redirect; an
	// empty company segment is a client error, not something to launder.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// respondStarted replies 202 with the started workflow's identifiers.
func respondStarted(w http.ResponseWriter, run client.WorkflowRun) {
	body, err := json.Marshal(map[string]string{
		"workflow_id": run.GetID(),
		"run_id":      run.GetRunID(),
	})
	if err != nil {
		http.Error(w, "encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(body)
}
