# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**maestro** is a distributed task orchestration system managing up to 9k agents per cluster.

- **Module**: `github.com/filanov/maestro`
- **Go Version**: 1.25.4
- **Status**: Phase 1 implementation complete

**Core Technologies:**
- PostgreSQL (state storage with ACID guarantees)
- gRPC (agent communication)
- Protocol Buffers (service definitions)

**Key Dependencies:**
- `google.golang.org/grpc` - gRPC framework
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/golang-migrate/migrate/v4` - Database migrations
- `github.com/google/uuid` - UUID generation
- `github.com/caarlos0/env/v10` - Configuration from environment

## Development Commands

### Building
```bash
# Build both server and agent binaries
make build

# Build manually
go build -o bin/server ./cmd/server
go build -o bin/agent ./cmd/agent
```

### Protocol Buffers
```bash
# Generate Go code from proto files
make proto

# Clean generated proto files
make clean
```

### Database Migrations
```bash
# Run migrations up
export MAESTRO_DB_URL="postgres://user:pass@localhost/maestro?sslmode=disable"
make migrate-up

# Rollback last migration
make migrate-down
```

### Running Services
```bash
# Start PostgreSQL (example using Docker)
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=maestro postgres:16

# Run server
export MAESTRO_DB_URL="postgres://maestro:maestro@localhost/maestro?sslmode=disable"
./bin/server

# Run agent (in another terminal)
export MAESTRO_SERVICE_HOST=localhost
export MAESTRO_CLUSTER_ID=<cluster-id>  # Get from creating a cluster
./bin/agent
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

**UI → Service (REST)**:
- Cluster management (CRUD)
- Task management (CRUD + reorder)
- Agent monitoring (list, view, delete)
- Execution history and status

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
cmd/server/          - Service entry point
cmd/agent/           - Agent entry point
internal/api/grpc/   - gRPC server for agents
internal/api/rest/   - REST API for UI (not yet implemented)
internal/models/     - Data models
internal/db/         - Database interface
internal/db/postgres/- PostgreSQL implementation
internal/engine/     - Task scheduler
internal/monitor/    - Health monitor
internal/cleaner/    - Cleanup service
internal/config/     - Configuration management
proto/agent/v1/      - Protocol buffers
migrations/          - Database migrations
api/                 - OpenAPI specs (not yet implemented)
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
- REST + OpenAPI (UI protocol - not yet implemented)
- Docker (not yet implemented)

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

### Phase 2 - REST API & Management (TODO)
- [ ] REST API implementation
- [ ] OpenAPI documentation
- [ ] Docker containers (Dockerfile.server, Dockerfile.agent)
- [ ] Multi-instance deployment support
- [ ] Integration tests

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
