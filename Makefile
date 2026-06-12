.PHONY: test test-integration lint up down

GOLANGCI_LINT := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
WORKFLOWCHECK := go.temporal.io/sdk/contrib/tools/workflowcheck@v0.4.0
ACTIONLINT    := github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

# Unit tests: no infrastructure required.
test:
	go test ./...

# Integration tests: require the Compose stack (make up).
test-integration:
	go test -tags=integration ./...

# Linters: golangci-lint (style/correctness), workflowcheck (Temporal
# workflow determinism), actionlint (GitHub Actions workflows).
lint:
	go run $(GOLANGCI_LINT) run --build-tags=integration ./...
	go run $(WORKFLOWCHECK) ./...
	go run $(ACTIONLINT)

# Start / stop the local stack (Temporal, Postgres+pgvector, Web UI).
up:
	docker compose up -d --wait

down:
	docker compose down -v
