# ── Linting ───────────────────────────────────────────────────────────────────
install-golangci-lint:
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.10.1

LINT_PATHS := ./pkg/... ./cmd/... ./services/item/...

lint: install-golangci-lint
	golangci-lint run $(LINT_PATHS)

lint-fix: install-golangci-lint
	golangci-lint run --fix $(LINT_PATHS)

# ── Swagger ───────────────────────────────────────────────────────────────────
install-swag:
	go install github.com/swaggo/swag/cmd/swag@latest

swagger-generate:
	$(shell go env GOPATH)/bin/swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

swagger: install-swag swagger-generate

# ── Server ────────────────────────────────────────────────────────────────────
# swagger is a prerequisite to ensure docs/swagger/ is regenerated before start.
run-api: swagger
	go run cmd/api/main.go

run-worker:
	go run cmd/worker/main.go

# ── Migrations ────────────────────────────────────────────────────────────────
# Run all service migrations in dependency order.
migrate: migrate-item

migrate-item:
	go run migrations/item/run.go

# ── sqlc ──────────────────────────────────────────────────────────────────────
build-sql:
	cd services/item && sqlc generate

sqlc: build-sql
