// The worker hosts techcheck's Temporal workflow and activity
// implementations.
package main

import (
	"errors"
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

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

	activities, err := buildActivities()
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
	w.RegisterActivity(activities)

	slog.Info("worker running", "task_queue", workflows.TaskQueue, "temporal", hostPort)
	return w.Run(worker.InterruptCh())
}

// buildActivities wires the research activities with real providers, or
// hermetic fakes when TECHCHECK_FAKE_PROVIDERS=1 (integration tests).
func buildActivities() (*research.Activities, error) {
	briefsDir := os.Getenv("TECHCHECK_BRIEFS_DIR")
	if briefsDir == "" {
		briefsDir = "briefs"
	}

	if os.Getenv("TECHCHECK_FAKE_PROVIDERS") == "1" {
		slog.Warn("running with fake LLM and search providers")
		return &research.Activities{
			LLM:       fakeLLM{},
			Searcher:  fakeSearcher{},
			BriefsDir: briefsDir,
		}, nil
	}

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	tavilyKey := os.Getenv("TAVILY_API_KEY")
	if anthropicKey == "" || tavilyKey == "" {
		return nil, errors.New("ANTHROPIC_API_KEY and TAVILY_API_KEY are required (or set TECHCHECK_FAKE_PROVIDERS=1)")
	}

	return &research.Activities{
		LLM:       llm.NewAnthropic(anthropicKey),
		Searcher:  &search.Tavily{APIKey: tavilyKey},
		BriefsDir: briefsDir,
	}, nil
}
