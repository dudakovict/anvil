# Load .env if present so make targets (run, migrate-*) see the same env as the app.
-include .env
export

DB_DSN ?= postgres://app:app@localhost:5432/app?sslmode=disable
MIGRATIONS_DIR := migrations
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Install dev tools (swag, goose, mockgen via go tool, golangci-lint binary)
	go get -tool github.com/swaggo/swag/cmd/swag@latest
	go get -tool github.com/pressly/goose/v3/cmd/goose@latest
	go get -tool go.uber.org/mock/mockgen@latest
	@command -v golangci-lint >/dev/null || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin

.PHONY: build
build: ## Build the api binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/api ./cmd/api

.PHONY: run
run: ## Run the api locally (reads .env)
	go run ./cmd/api

.PHONY: test
test: ## Run tests with race detector and coverage
	go test -race -cover ./...

.PHONY: test-cover
test-cover: ## Run tests and open the HTML coverage report
	go test -race -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

.PHONY: audit
audit: ## Run all quality checks (mod tidy diff, verify, tests, lint, vulncheck)
	go mod tidy -diff
	go mod verify
	go test -race ./...
	golangci-lint run
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: upgradeable
upgradeable: ## List direct dependencies with upgrades available
	go run github.com/oligot/go-mod-upgrade@latest

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format code, imports and swagger annotations (swag fmt + gofumpt + goimports)
	go tool swag fmt
	golangci-lint fmt

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: swagger
swagger: ## Regenerate swagger docs from annotations
	go tool swag init -g cmd/api/main.go -o docs

.PHONY: mocks
mocks: ## Regenerate gomock mocks for the port interfaces
	go tool mockgen -destination=internal/core/port/mock/mock.go github.com/dudakovict/anvil/internal/core/port UserService,UserRepository

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down

.PHONY: migrate-status
migrate-status: ## Show migration status
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" status

.PHONY: migrate-create
migrate-create: ## Create a new migration: make migrate-create name=add_users
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

.PHONY: docker-build
docker-build: ## Build the docker image
	docker build --build-arg VERSION=$(VERSION) -t anvil .

.PHONY: docker-up
docker-up: ## Start app + postgres via docker compose
	docker compose up --build -d

.PHONY: docker-watch
docker-watch: ## Start the stack and rebuild the app on source changes
	docker compose up --build --watch

.PHONY: docker-down
docker-down: ## Stop compose stack and remove volumes
	docker compose down -v

.PHONY: db-up
db-up: ## Start only postgres (for host-run development)
	docker compose up -d postgres
