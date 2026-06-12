// The worker hosts techcheck's Temporal workflow and activity
// implementations.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/campoy/techcheck/internal/corpus"
	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
	"github.com/campoy/techcheck/internal/workflows"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	hostPort := os.Getenv("TEMPORAL_HOSTPORT")
	if hostPort == "" {
		hostPort = client.DefaultHostPort
	}

	researchActs, ingestActs, err := buildActivities()
	if err != nil {
		return err
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, workflows.TaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.Hello)
	w.RegisterActivity(workflows.SayHello)
	w.RegisterWorkflow(research.CompanyResearch)
	w.RegisterActivity(researchActs)
	w.RegisterWorkflow(corpus.IngestCorpus)
	w.RegisterActivity(ingestActs)

	slog.Info("worker running", "task_queue", workflows.TaskQueue, "temporal", hostPort)
	return w.Run(worker.InterruptCh())
}

// buildActivities wires the research and ingestion activities with real
// providers, or hermetic fakes when TECHCHECK_FAKE_PROVIDERS=1 (integration
// tests). The corpus store is real either way — it is part of the local
// open-source stack — only the embedder is faked.
func buildActivities() (*research.Activities, *corpus.Activities, error) {
	briefsDir := os.Getenv("TECHCHECK_BRIEFS_DIR")
	if briefsDir == "" {
		briefsDir = "briefs"
	}

	dsn := os.Getenv("TECHCHECK_PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/research?sslmode=disable"
	}
	ctx := context.Background()
	store, err := corpus.NewStore(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		return nil, nil, err
	}

	if os.Getenv("TECHCHECK_FAKE_PROVIDERS") == "1" {
		slog.Warn("running with fake LLM, search, and embedding providers")
		cor := &corpus.Corpus{Store: store, Embedder: corpus.LexicalEmbedder{}}
		return &research.Activities{
			LLM:       fakeLLM{},
			Searcher:  fakeSearcher{},
			Corpus:    cor,
			BriefsDir: briefsDir,
		}, &corpus.Activities{Corpus: cor}, nil
	}

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	tavilyKey := os.Getenv("TAVILY_API_KEY")
	voyageKey := os.Getenv("VOYAGE_API_KEY")
	if anthropicKey == "" || tavilyKey == "" || voyageKey == "" {
		return nil, nil, errors.New("ANTHROPIC_API_KEY, TAVILY_API_KEY, and VOYAGE_API_KEY are required (or set TECHCHECK_FAKE_PROVIDERS=1)")
	}

	cor := &corpus.Corpus{Store: store, Embedder: &corpus.Voyage{APIKey: voyageKey}}
	return &research.Activities{
		LLM:       llm.NewAnthropic(anthropicKey),
		Searcher:  &search.Tavily{APIKey: tavilyKey},
		Corpus:    cor,
		BriefsDir: briefsDir,
	}, &corpus.Activities{Corpus: cor}, nil
}
