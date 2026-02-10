# Maestro REST API

## Overview

The Maestro REST API provides management operations for clusters, agents, tasks, and executions. It runs on port 8080 by default and complements the gRPC API used by agents.

## Base URL

```
http://localhost:8080
```

## Authentication

Currently no authentication (to be added in future phases).

## Endpoints

### Health Check

**GET /health**

Returns the health status of the service.

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok"
}
```

---

### Clusters

#### List Clusters

**GET /api/v1/clusters**

Query parameters:
- `limit` (optional, default: 50, max: 1000)
- `offset` (optional, default: 0)

```bash
curl http://localhost:8080/api/v1/clusters?limit=10&offset=0
```

Response:
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "production",
      "description": "Production cluster",
      "created_at": "2026-02-10T12:00:00Z",
      "updated_at": "2026-02-10T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

#### Create Cluster

**POST /api/v1/clusters**

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production",
    "description": "Production cluster"
  }'
```

#### Get Cluster

**GET /api/v1/clusters/{id}**

```bash
curl http://localhost:8080/api/v1/clusters/{cluster-id}
```

#### Delete Cluster

**DELETE /api/v1/clusters/{id}**

```bash
curl -X DELETE http://localhost:8080/api/v1/clusters/{cluster-id}
```

---

### Agents

#### List Agents by Cluster

**GET /api/v1/clusters/{clusterId}/agents**

Query parameters:
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
curl http://localhost:8080/api/v1/clusters/{cluster-id}/agents
```

#### Get Agent

**GET /api/v1/agents/{id}**

```bash
curl http://localhost:8080/api/v1/agents/{agent-id}
```

#### Delete Agent

**DELETE /api/v1/agents/{id}**

```bash
curl -X DELETE http://localhost:8080/api/v1/agents/{agent-id}
```

#### List Agent Executions

**GET /api/v1/agents/{id}/executions**

Query parameters:
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
curl http://localhost:8080/api/v1/agents/{agent-id}/executions
```

---

### Tasks

#### List Tasks

**GET /api/v1/tasks**

Query parameters:
- `cluster_id` (required)
- `include_deleted` (optional, default: false)
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
curl "http://localhost:8080/api/v1/tasks?cluster_id={cluster-id}"
```

#### Create Task

**POST /api/v1/tasks**

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": "uuid",
    "name": "install-packages",
    "type": "exec",
    "blocking": true,
    "config": {
      "command": "apt-get update && apt-get install -y curl",
      "timeout_seconds": 300,
      "working_dir": "/tmp"
    }
  }'
```

#### Get Task

**GET /api/v1/tasks/{id}**

```bash
curl http://localhost:8080/api/v1/tasks/{task-id}
```

#### Update Task

**PUT /api/v1/tasks/{id}**

```bash
curl -X PUT http://localhost:8080/api/v1/tasks/{task-id} \
  -H "Content-Type: application/json" \
  -d '{
    "name": "updated-task-name",
    "blocking": false
  }'
```

#### Delete Task

**DELETE /api/v1/tasks/{id}**

Soft deletes the task (sets deleted_at timestamp).

```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/{task-id}
```

#### Reorder Tasks

**POST /api/v1/tasks/reorder**

Atomically reorders all tasks for a cluster.

```bash
curl -X POST http://localhost:8080/api/v1/tasks/reorder \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": "uuid",
    "task_ids": ["task-uuid-1", "task-uuid-2", "task-uuid-3"]
  }'
```

---

### Executions

#### List Executions

**GET /api/v1/executions**

Query parameters:
- `cluster_id` (optional)
- `agent_id` (optional)
- `task_id` (optional)
- `status` (optional: pending, running, success, failed, skipped)
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
# List all executions for a cluster
curl "http://localhost:8080/api/v1/executions?cluster_id={cluster-id}"

# List failed executions
curl "http://localhost:8080/api/v1/executions?status=failed"

# List executions for specific agent
curl "http://localhost:8080/api/v1/executions?agent_id={agent-id}"
```

#### Get Execution

**GET /api/v1/executions/{id}**

```bash
curl http://localhost:8080/api/v1/executions/{execution-id}
```

---

### Debug Tasks

#### Create Debug Task

**POST /api/v1/debug-tasks**

Creates a one-off command to be executed by a specific agent.

```bash
curl -X POST http://localhost:8080/api/v1/debug-tasks \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": "uuid",
    "agent_id": "uuid",
    "command": "uptime"
  }'
```

#### Get Debug Task

**GET /api/v1/debug-tasks/{id}**

```bash
curl http://localhost:8080/api/v1/debug-tasks/{debug-task-id}
```

#### List Debug Tasks by Agent

**GET /api/v1/agents/{agentId}/debug-tasks**

Query parameters:
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
curl http://localhost:8080/api/v1/agents/{agent-id}/debug-tasks
```

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "Bad Request",
  "message": "name is required"
}
```

Common HTTP status codes:
- `200 OK` - Successful GET/PUT
- `201 Created` - Successful POST
- `204 No Content` - Successful DELETE
- `400 Bad Request` - Invalid request body or parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Pagination

All list endpoints support pagination:

```bash
curl "http://localhost:8080/api/v1/clusters?limit=10&offset=20"
```

Response includes pagination metadata:
```json
{
  "data": [...],
  "total": 100,
  "limit": 10,
  "offset": 20
}
```

---

## Configuration

REST API configuration via environment variables:

- `MAESTRO_REST_HOST` - Bind address (default: `0.0.0.0`)
- `MAESTRO_REST_PORT` - Port (default: `8080`)

---

## Architecture

The REST API is built using:
- **Router**: `github.com/go-chi/chi/v5` - Lightweight, fast HTTP router
- **Middleware**: Logging, recovery, request ID tracking, CORS
- **DTOs**: Dedicated data transfer objects separate from internal models
- **Handlers**: Organized by resource type (clusters, agents, tasks, etc.)
- **Error Handling**: Consistent error responses with proper HTTP status codes

The REST server runs concurrently with the gRPC server and supports graceful shutdown.
