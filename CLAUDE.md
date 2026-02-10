# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**maestro** is a distributed task orchestration system managing up to 9k agents per cluster.

- **Module**: `github.com/filanov/maestro`
- **Go Version**: 1.25.4
- **Status**: Phase 2 REST API complete

**Core Technologies:**
- PostgreSQL (state storage with ACID guarantees)
- gRPC (agent communication)
- REST API (management operations)
- Protocol Buffers (service definitions)

**Key Dependencies:**
- `google.golang.org/grpc` - gRPC framework
- `github.com/go-chi/chi/v5` - HTTP router for REST API
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/golang-migrate/migrate/v4` - Database migrations
- `github.com/google/uuid` - UUID generation
- `github.com/caarlos0/env/v10` - Configuration from environment

## Development Commands

**All commands run inside Docker containers for consistency across environments.**

### Quick Start
```bash
# Get help
make help

# Build everything (runs in container)
make all

# Start development environment
make docker-up
```

### Building (Containerized)
```bash
# Build both server and agent binaries (in container)
make build

# Generate protobuf files (in container)
make proto

# Clean generated files
make clean

# Build Docker images for deployment
make docker-build
```

### Testing (Containerized)
```bash
# Run all tests (unit + integration with PostgreSQL)
make test

# Run only unit tests (fast, no DB required)
make test-unit

# Run integration tests (with PostgreSQL)
make test-integration

# Generate coverage report
make test-coverage
```

### Code Quality (Containerized)
```bash
# Run all linters
make lint

# Format code
make fmt

# Check formatting
make fmt-check
```

### Database Operations
```bash
# Start PostgreSQL containers
make docker-up

# Run migrations
make migrate-up

# Rollback last migration
make migrate-down

# View database logs
make docker-logs
```

### Running Services
```bash
# Run server (starts postgres, runs migrations, starts server)
make run-server

# Open shell in build container
make shell

# View running containers
make docker-ps

# Stop all containers
make docker-down
```

### Docker Infrastructure
```bash
# Build development container image
make docker-build-image

# Start all services (postgres, test DB)
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...

# Run a specific test
go test -v -run TestName ./path/to/package
```

### Code Quality
```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture

### Overview

Maestro is a distributed task orchestration system managing up to 9k agents per cluster.

**Key Design Principles**:
- **Pull-based architecture**: Agents poll for tasks (service doesn't push)
- **Deterministic agent IDs**: UUID v5 generated from hostname
- **Task reset on re-registration**: Agent re-registration triggers full task reset
- **Sequential execution**: Agents execute one task at a time
- **Blocking tasks**: Failed blocking task skips all subsequent tasks

### Communication Model

**Agent → Service (gRPC)**:
- `Register(cluster_id, hostname)` - Returns deterministic agent_id
- `Heartbeat(agent_id)` - Every 1 minute
- `PollTasks(agent_id)` - When idle or after task completion
- `ReportTaskExecution(agent_id, task_id, status, output)` - After each task
- `PollDebugTasks(agent_id)` - Every 5 seconds

**UI → Service (REST API)**:
- **Clusters**: List, Create, Get, Delete
- **Agents**: List (by cluster), Get, Delete, List executions
- **Tasks**: List, Create, Get, Update, Delete, Reorder
- **Executions**: List (with filtering), Get
- **Debug Tasks**: Create, Get, List (by agent)
- **Health**: Health check endpoint

**REST API Base URL**: `http://localhost:8080`

All list endpoints support pagination via `?limit=50&offset=0` query parameters.

### Core Components

**Task Scheduler** (`internal/engine/scheduler.go`):
- Returns ordered list of pending tasks for agent
- Filters out completed tasks
- Handles blocking task logic
- Marks tasks as skipped when blocked

**Health Monitor** (`internal/monitor/health.go`):
- Checks agent heartbeat every minute
- Marks agent disconnected after 3 missed heartbeats (3 minutes)
- Fails running tasks for disconnected agents

**DB** (`internal/db/postgres/`):
- PostgreSQL for persistence
- Agent executions tracked per task
- Unique constraint prevents duplicate executions

### Data Flow

**Agent Startup**:
```
1. Register(cluster_id, hostname) → agent_id, reset_flag
2. If reset=true: all previous executions deleted
3. Start heartbeat loop (every 1 minute)
4. Start task execution loop:
   - PollTasks → get task list
   - If empty: sleep 1 minute
   - Execute each task sequentially
   - Report results
   - Poll again immediately
```

**Task Scheduling Logic**:
```
1. Get all cluster tasks (ordered)
2. Get agent's successful executions
3. For each task:
   - Skip if executed successfully
   - Break and skip remaining if blocked by failed blocking task
   - Skip if task failed (non-blocking)
   - Otherwise include in pending list
4. Return pending list
```

### Package Structure

```
cmd/server/                - Service entry point
cmd/agent/                 - Agent entry point
internal/api/grpc/         - gRPC server for agents
internal/api/rest/         - REST API for UI/management
  ├── handlers/            - HTTP request handlers
  │   ├── clusters.go      - Cluster CRUD endpoints
  │   ├── agents.go        - Agent management endpoints
  │   ├── tasks.go         - Task CRUD + reorder endpoints
  │   ├── executions.go    - Execution listing/viewing
  │   ├── debug_tasks.go   - Debug task management
  │   └── health.go        - Health check
  ├── dto/                 - Data transfer objects
  ├── middleware/          - HTTP middleware (logging, recovery, CORS)
  ├── server.go            - REST server setup
  ├── router.go            - Route definitions
  └── errors.go            - Error handling utilities
internal/models/           - Data models
internal/db/               - Database interface
internal/db/postgres/      - PostgreSQL implementation
internal/engine/           - Task scheduler
internal/monitor/          - Health monitor
internal/cleaner/          - Cleanup service
internal/config/           - Configuration management
proto/agent/v1/            - Protocol buffers
migrations/                - Database migrations
api/                       - OpenAPI specs (not yet implemented)
```

### Database Schema

**Key tables**:
- `clusters` - Cluster definitions
- `agents` - Agent registry (id from hostname, tracks heartbeat)
- `tasks` - Ordered task list per cluster
- `task_executions` - Execution history per agent per task
- `debug_tasks` - One-off commands for specific agents

**Important constraints**:
- `UNIQUE(cluster_id, order)` on tasks - enforces task ordering
- `UNIQUE(cluster_id, hostname)` on agents - one agent per hostname per cluster
- `UNIQUE(agent_id, task_id)` on executions - prevents duplicate execution

### Technology Stack

- Go 1.25.4
- PostgreSQL (ordered tasks, ACID guarantees)
- gRPC (agent protocol)
- REST API with chi router (management operations)
- Docker (containerized development)

## Implementation Status

### Phase 1 - Core Foundation ✅ COMPLETED
- [x] Project setup (go mod, proto compilation, Makefile)
- [x] Protocol buffer definitions (gRPC service)
- [x] Data models (Cluster, Agent, Task, Execution, DebugTask)
- [x] Database migrations (PostgreSQL schema)
- [x] DB interface and PostgreSQL implementation
- [x] Configuration management (environment variables)
- [x] Task scheduler engine (ordered execution logic)
- [x] gRPC server (all 6 RPC methods)
- [x] Health monitor (heartbeat tracking)
- [x] Cleaner service (execution history cleanup)
- [x] Service main binary (cmd/server)
- [x] Agent implementation (cmd/agent)
- [x] Graceful shutdown (both service and agent)

**Binaries built:**
- `bin/server` - Maestro orchestration service
- `bin/agent` - Agent for task execution

### Phase 2 - REST API & Management ✅ COMPLETED
- [x] REST API implementation (24 endpoints)
  - [x] Cluster management (4 endpoints: List, Create, Get, Delete)
  - [x] Agent management (4 endpoints: List by cluster, Get, Delete, List executions)
  - [x] Task management (6 endpoints: List, Create, Get, Update, Delete, Reorder)
  - [x] Execution viewing (3 endpoints: List with filtering, Get, List by agent)
  - [x] Debug task management (3 endpoints: Create, Get, List by agent)
  - [x] Health check (1 endpoint)
  - [x] Request/response logging middleware
  - [x] Error handling with proper HTTP status codes
  - [x] Pagination support on all list endpoints
  - [x] Graceful shutdown alongside gRPC server
- [ ] OpenAPI documentation
- [ ] Docker containers (Dockerfile.server, Dockerfile.agent)
- [ ] Multi-instance deployment support

### Phase 3 - Observability & Testing (TODO)
- [ ] Metrics (Prometheus)
- [ ] Structured logging improvements
- [ ] Performance testing with 9k agents
- [ ] Load testing
- [ ] E2E tests

### Phase 4 - UI & Polish (TODO)
- [ ] Web UI
- [ ] CLI tool
- [ ] Documentation
- [ ] Deployment guides
