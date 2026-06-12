// The api binary serves techcheck's REST/JSON API, translating HTTP calls
// into Temporal client operations.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"go.temporal.io/sdk/client"

	"github.com/campoy/techcheck/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	hostPort := os.Getenv("TEMPORAL_HOSTPORT")
	if hostPort == "" {
		hostPort = client.DefaultHostPort
	}
	addr := os.Getenv("TECHCHECK_API_ADDR")
	if addr == "" {
		addr = "localhost:8090" // localhost by default: single-user tool
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return err
	}
	defer c.Close()

	slog.Info("api listening", "addr", addr, "temporal", hostPort)
	return http.ListenAndServe(addr, server.New(c))
}
