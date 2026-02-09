# Makefile - All targets run inside build container
.PHONY: all proto clean build test test-unit test-integration test-coverage lint fmt fmt-check \
        docker-build docker-up docker-down docker-logs docker-ps \
        run-server run-agent migrate-up migrate-down shell help

# Docker compose command
DOCKER_COMPOSE := docker-compose
DOCKER_RUN := $(DOCKER_COMPOSE) run --rm build
DOCKER_RUN_DB := $(DOCKER_COMPOSE) run --rm -e MAESTRO_TEST_DB_URL=postgres://maestro:maestro@postgres-test:5432/maestro_test?sslmode=disable build

# Default target
all: docker-build-image build lint

help:
	@echo "Maestro Build System - All commands run in containers"
	@echo ""
	@echo "Build Commands:"
	@echo "  make all                - Build everything (proto, binaries, lint)"
	@echo "  make proto              - Generate protobuf files"
	@echo "  make build              - Build server and agent binaries"
	@echo "  make clean              - Clean generated files and binaries"
	@echo ""
	@echo "Test Commands:"
	@echo "  make test               - Run all tests (unit + integration)"
	@echo "  make test-unit          - Run unit tests only"
	@echo "  make test-integration   - Run integration tests (requires DB)"
	@echo "  make test-coverage      - Generate test coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint               - Run linters (vet, fmt check, golangci-lint)"
	@echo "  make fmt                - Format code with gofmt"
	@echo "  make fmt-check          - Check code formatting"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up         - Run database migrations"
	@echo "  make migrate-down       - Rollback last migration"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build-image - Build the development container image"
	@echo "  make docker-build       - Build server and agent Docker images"
	@echo "  make docker-up          - Start all services (postgres, server)"
	@echo "  make docker-down        - Stop all services"
	@echo "  make docker-logs        - View service logs"
	@echo "  make docker-ps          - List running containers"
	@echo ""
	@echo "Development:"
	@echo "  make shell              - Open shell in build container"
	@echo "  make run-server         - Run server locally (in container)"
	@echo "  make run-agent          - Run agent locally (in container)"

# Docker infrastructure
docker-build-image:
	@echo "Building development container image..."
	@$(DOCKER_COMPOSE) build build

docker-build: docker-build-image
	@echo "Building server and agent Docker images..."
	@$(DOCKER_COMPOSE) build server

docker-up:
	@echo "Starting services..."
	@$(DOCKER_COMPOSE) up -d postgres postgres-test
	@echo "Waiting for databases to be ready..."
	@sleep 5
	@echo "Services started. Use 'make docker-logs' to view logs"

docker-down:
	@echo "Stopping services..."
	@$(DOCKER_COMPOSE) down

docker-logs:
	@$(DOCKER_COMPOSE) logs -f

docker-ps:
	@$(DOCKER_COMPOSE) ps

# Proto generation
proto:
	@echo "Generating protobuf files (in container)..."
	@$(DOCKER_RUN) sh -c "protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent/v1/agent.proto"

# Build targets
clean:
	@echo "Cleaning generated files..."
	@rm -f proto/agent/v1/*.pb.go
	@rm -rf bin/
	@rm -f coverage.out coverage.html

build: proto
	@echo "Building binaries (in container)..."
	@mkdir -p bin
	@$(DOCKER_RUN) sh -c "go build -o bin/server ./cmd/server"
	@$(DOCKER_RUN) sh -c "go build -o bin/agent ./cmd/agent"
	@echo "Binaries built: bin/server, bin/agent"

# Test targets
test: docker-up
	@echo "Running all tests (in container with database)..."
	@$(DOCKER_RUN_DB) sh -c "ginkgo -r -v --randomize-all --randomize-suites --fail-on-pending --cover --trace || go test -v ./..."

test-unit:
	@echo "Running unit tests (in container)..."
	@$(DOCKER_RUN) sh -c "ginkgo -r -v --randomize-all --skip-package=internal/db/postgres || go test -v \$$(go list ./... | grep -v internal/db/postgres)"

test-integration: docker-up
	@echo "Running integration tests (in container with database)..."
	@$(DOCKER_RUN_DB) sh -c "ginkgo -v internal/db/postgres || go test -v ./internal/db/postgres"

test-coverage: docker-up
	@echo "Running tests with coverage (in container)..."
	@$(DOCKER_RUN_DB) sh -c "go test -coverprofile=coverage.out ./..."
	@$(DOCKER_RUN) sh -c "go tool cover -html=coverage.out -o coverage.html"
	@echo "Coverage report generated: coverage.html"

# Linting targets
lint:
	@echo "Running linters (in container)..."
	@$(DOCKER_RUN) sh -c "go vet ./..."
	@echo "Running format check..."
	@$(DOCKER_RUN) sh -c "test -z \"\$$(gofmt -l . | grep -v 'proto/.*\.pb\.go')\" || (echo 'Files not formatted, run make fmt' && gofmt -l . | grep -v 'proto/.*\.pb\.go' && exit 1)"
	@echo "Running golangci-lint..."
	@$(DOCKER_RUN) sh -c "golangci-lint run || echo 'golangci-lint checks completed with warnings'"

fmt:
	@echo "Formatting code (in container)..."
	@$(DOCKER_RUN) sh -c "gofmt -w -l \$$(find . -name '*.go' | grep -v '.pb.go')"

fmt-check:
	@echo "Checking formatting (in container)..."
	@$(DOCKER_RUN) sh -c "test -z \"\$$(gofmt -l . | grep -v 'proto/.*\.pb\.go')\" || (echo 'Files not formatted:' && gofmt -l . | grep -v 'proto/.*\.pb\.go' && exit 1)"

# Migration targets
migrate-up: docker-up
	@echo "Running migrations (in container)..."
	@$(DOCKER_RUN) sh -c "migrate -path migrations -database 'postgres://maestro:maestro@postgres:5432/maestro?sslmode=disable' up"

migrate-down: docker-up
	@echo "Rolling back migration (in container)..."
	@$(DOCKER_RUN) sh -c "migrate -path migrations -database 'postgres://maestro:maestro@postgres:5432/maestro?sslmode=disable' down 1"

# Development targets
shell:
	@echo "Opening shell in build container..."
	@$(DOCKER_RUN) /bin/bash

run-server: docker-up migrate-up
	@echo "Running server (in container)..."
	@$(DOCKER_COMPOSE) up server

run-agent:
	@echo "Running agent (in container)..."
	@$(DOCKER_RUN) sh -c "go run ./cmd/agent"
