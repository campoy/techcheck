.PHONY: test test-integration up down

# Unit tests: no infrastructure required.
test:
	go test ./...

# Integration tests: require the Compose stack (make up).
test-integration:
	go test -tags=integration ./...

# Start / stop the local stack (Temporal, Postgres+pgvector, Web UI).
up:
	docker compose up -d --wait

down:
	docker compose down -v
