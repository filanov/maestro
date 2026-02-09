# Maestro - Distributed Task Orchestration System

A distributed task orchestration system for managing up to 9,000 agents per cluster, executing ordered bash commands with blocking/non-blocking semantics.

## Features

- **Pull-based architecture**: Agents poll for work (no push from service)
- **Deterministic agent IDs**: UUID v5 generated client-side from cluster_id + hostname
- **Sequential task execution**: Agents execute one task at a time
- **Blocking task semantics**: Failed blocking tasks skip all subsequent tasks
- **Health monitoring**: Automatic detection of inactive agents (3 missed heartbeats)
- **Stateless service**: Multiple instances supported with PostgreSQL backend
- **gRPC communication**: Efficient binary protocol for agent-service communication

## Containerized Build System

**All development commands run inside Docker containers** for consistency across environments.  
**No local Go installation required!**

### Prerequisites

- Docker and Docker Compose
- Make (standard on macOS/Linux)

### Quick Start

```bash
# View all available commands
make help

# Build everything (proto, binaries, lint)
make all

# Run tests
make test-unit

# Start development environment
make docker-up
make test
```

## Make Targets

All targets run in containers. Common commands:

```bash
make build              # Build binaries
make test              # Run all tests (with DB)
make test-unit         # Run unit tests only
make lint              # Run linters
make fmt               # Format code
make docker-up         # Start PostgreSQL
make migrate-up        # Run migrations
make run-server        # Run server locally
make shell             # Open container shell
make help              # Show all commands
```

## Project Structure

```
maestro/
├── cmd/               # Server & Agent binaries
├── internal/          # Core implementation
│   ├── api/grpc/      # gRPC server
│   ├── db/postgres/   # Database layer
│   ├── engine/        # Task scheduler
│   └── models/        # Data models
├── proto/             # Protocol Buffers
├── migrations/        # Database migrations
├── Dockerfile.build   # Dev container
├── docker-compose.yml # Dev environment
└── Makefile          # All in containers
```

## Development Workflow

### 1. Build
```bash
make build          # Compiles in container
# Output: bin/server, bin/agent
```

### 2. Test
```bash
make test-unit      # Fast unit tests
make docker-up      # Start databases
make test           # All tests + integration
make test-coverage  # Generate coverage.html
```

### 3. Run Locally
```bash
make docker-up      # Start PostgreSQL
make run-server     # Run service
# In another terminal:
make shell          # Open container
go run ./cmd/agent  # Run agent
```

## Architecture

```
Service (stateless, multiple instances)
  ├─ gRPC API (agents communicate)
  ├─ Health Monitor (track heartbeats)
  └─ Cleaner (remove old data)
       │
       ▼
  PostgreSQL (all state)
       ▲
       │
  Agents (up to 9k)
  ├─ Register
  ├─ Heartbeat (1 min)
  ├─ Poll tasks
  └─ Execute & report
```

## Configuration

### Service
```bash
MAESTRO_DB_URL="postgres://user:pass@host/db?sslmode=disable"
MAESTRO_GRPC_PORT="9090"
```

### Agent  
```bash
MAESTRO_SERVICE_HOST="server"
MAESTRO_CLUSTER_ID="cluster-uuid"
```

## CI/CD

All commands run in containers:

```yaml
- run: make all
- run: make test
- run: make docker-build
```

## License

MIT
