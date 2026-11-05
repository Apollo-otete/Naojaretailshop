# Naoja Retail Shop — Go API
#
# `make check` is the Phase 2 exit criterion ("deployable API skeleton with automated checks")
# expressed as one command: fmt, vet, unit tests. CI runs exactly that, so anything that passes
# locally passes upstream without a second toolchain to keep in sync.

# Pinned deliberately. The installed toolchain is go1.22.2; without this, `go get` happily
# rewrote the go directive to a version this machine cannot run, and a dependency
# (rogpeppe/go-internal) demanded go>=1.25. Refusing to download a toolchain keeps the build
# honest about what is actually supported.
export GOTOOLCHAIN := local

BIN           := bin
VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -s -w -X main.version=$(VERSION)
DATABASE_URL  ?= postgres://naoja:naoja_dev_only@localhost:5433/naoja?sslmode=disable
export DATABASE_URL

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Run & build ────────────────────────────────────────────────────────────────────────────

.PHONY: run
run: ## run the API locally (expects backend-go/.env or an exported DATABASE_URL)
	go run ./cmd/api

.PHONY: build
build: ## compile the API and the migrate tool into ./bin
	@mkdir -p $(BIN)
	go build -ldflags '$(LDFLAGS)' -o $(BIN)/api ./cmd/api
	go build -ldflags '$(LDFLAGS)' -o $(BIN)/migrate ./cmd/migrate

# ── Automated checks ───────────────────────────────────────────────────────────────────────

.PHONY: fmt
fmt: ## rewrite sources with gofmt
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## fail if anything is unformatted (CI gate; does not touch files)
	@out=$$(gofmt -s -l .); \
	if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## run go vet
	go vet ./...

.PHONY: test
test: ## run unit tests only — no database, no network
	go test ./... -count=1

.PHONY: test-race
test-race: ## unit tests under the race detector
	go test -race ./... -count=1

# The migration suite drops every schema, so it gets its own database rather than sharing the
# one `make run` uses. db-up creates both. Override for CI with an explicit URL whose database
# name ends in _test — that variable is also how a non-loopback host is permitted.
INTEGRATION_DATABASE_URL ?= postgres://naoja:naoja_dev_only@localhost:5433/naoja_test?sslmode=disable

.PHONY: test-integration
test-integration: ## run tests that need a live PostgreSQL (see db-up)
	MIGRATION_TEST_DATABASE_URL=$(INTEGRATION_DATABASE_URL) go test -tags=integration ./... -count=1

.PHONY: cover
cover: ## unit-test coverage profile + per-package totals
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: check
check: fmt-check vet test ## everything CI requires

# ── Migrations (golang-migrate, SQL embedded from migrations/sql) ──────────────────────────

.PHONY: migrate-up
migrate-up: ## apply every pending migration
	go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## roll back one migration (override with N=3)
	go run ./cmd/migrate down $(or $(N),1)

.PHONY: migrate-down-all
migrate-down-all: ## roll back every applied migration
	go run ./cmd/migrate down -all

.PHONY: migrate-status
migrate-status: ## report current version, dirtiness and what is pending
	go run ./cmd/migrate status

.PHONY: migrate-force
migrate-force: ## mark V=… applied without running its SQL (needs YES=1)
	@test -n "$(V)" || { echo "usage: make migrate-force V=2 YES=1"; exit 1; }
	@test "$(YES)" = "1" || { echo "refusing: this rewrites history without executing it. Add YES=1"; exit 1; }
	go run ./cmd/migrate -yes force $(V)

# ── Local database ─────────────────────────────────────────────────────────────────────────

.PHONY: db-up
db-up: ## start PostgreSQL on :5433 and wait until it answers a query
	docker compose -f docker-compose.yml up -d --wait postgres

.PHONY: db-down
db-down: ## stop PostgreSQL, keeping the data volume
	docker compose -f docker-compose.yml down

.PHONY: db-reset
db-reset: ## destroy the local volume and start clean — local data only, never a production tool
	docker compose -f docker-compose.yml down -v

.PHONY: tidy
tidy: ## sync go.mod/go.sum
	go mod tidy
