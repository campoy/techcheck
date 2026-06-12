// The worker hosts techcheck's Temporal workflow and activity
// implementations.
package main

import (
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

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

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, workflows.TaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.Hello)
	w.RegisterActivity(workflows.SayHello)

	slog.Info("worker running", "task_queue", workflows.TaskQueue, "temporal", hostPort)
	return w.Run(worker.InterruptCh())
}
