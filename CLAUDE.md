# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**maestro** is a Go project currently in initial setup phase.

- **Module**: `github.com/filanov/maestro`
- **Go Version**: 1.25.4
- **License**: MIT

## Development Commands

### Building
```bash
go build ./...
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...

# Run a specific test
go test -v -run TestName ./path/to/package

# View coverage report
go tool cover -html=coverage.out
```

### Formatting and Linting
```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Run golangci-lint (if installed)
golangci-lint run
```

### Dependency Management
```bash
# Add/update dependencies
go get <package>

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify

# Vendor dependencies (if needed)
go mod vendor
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

**Store** (`internal/store/postgres/`):
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
internal/api/rest/   - REST API for UI
internal/models/     - Data models
internal/store/      - PostgreSQL store
internal/engine/     - Task scheduler
internal/monitor/    - Health monitor
proto/agent/v1/      - Protocol buffers
api/                 - OpenAPI specs
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
- REST + OpenAPI (UI protocol)
- Docker
