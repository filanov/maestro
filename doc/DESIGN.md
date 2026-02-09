# Maestro - Design Document

## Overview

Maestro is a distributed task orchestration system that manages clusters of agents (up to 9k per cluster), coordinating ordered task execution across them.

**Key Characteristics**:
- Stateless service (multiple instances supported)
- Pull-based architecture (agents poll for work)
- PostgreSQL for all state
- gRPC for agent communication
- REST API for management

## Architecture

```
        ┌──────────────┐
        │ Load Balancer│ (REST traffic)
        └──────┬───────┘
               │
       ┌───────┴────────┐
       │                │
┌──────▼──────┐  ┌─────▼───────┐
│  Service 1  │  │  Service 2  │  (Multiple instances)
│  - gRPC     │  │  - gRPC     │
│  - REST     │  │  - REST     │
│  - Monitor  │  │  - Monitor  │
│  - Cleaner  │  │  - Cleaner  │
└──────┬──────┘  └─────┬───────┘
       │                │
       └────────┬───────┘
                │
         ┌──────▼──────┐
         │  PostgreSQL │
         │  (All State)│
         └─────────────┘
                ▲
                │
    ┌───────────┴───────────┐
    │                       │
┌───▼────┐  ┌─────────┐  ┌─▼──────┐
│Agent 1 │  │ Agent...│  │Agent 9k│
└────────┘  └─────────┘  └────────┘
```

## Data Model

### Cluster
```go
type Cluster struct {
    ID          string    // UUID
    Name        string
    Description string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Agent
```go
type Agent struct {
    ID              string    // UUID (agent computes from cluster_id + hostname)
    ClusterID       string
    Hostname        string    // Agent hostname (unique per cluster)
    Status          AgentStatus // active, inactive
    LastHeartbeat   time.Time
    RegisteredAt    time.Time
    LastResetAt     time.Time  // When tasks were last reset
}

type AgentStatus string
const (
    AgentStatusActive   AgentStatus = "active"
    AgentStatusInactive AgentStatus = "inactive"
)
```

**Agent ID Generation** (Client-Side):
```go
// Agent calculates its own ID
func GenerateAgentID(clusterID, hostname string) string {
    namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // DNS namespace
    data := clusterID + ":" + hostname
    return uuid.NewSHA1(namespace, []byte(data)).String()
}
```

### Task
```go
type Task struct {
    ID          string
    ClusterID   string
    Name        string
    Type        TaskType  // exec
    Order       int       // Execution order (unique per cluster, active tasks only)
    Blocking    bool      // Failure stops subsequent tasks
    Config      TaskConfig
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time // Soft delete
}

type TaskType string
const (
    TaskTypeExec TaskType = "exec"
)

type TaskConfig struct {
    Command     string
    Timeout     time.Duration // Default: 30min, Max: 24h, Min: 10s
    WorkingDir  string        // Default: "" (agent's CWD), must be absolute if specified
}
```

### TaskExecution
```go
type TaskExecution struct {
    ID          string
    TaskID      string
    AgentID     string
    Status      ExecutionStatus
    Output      string    // stdout/stderr (max 1MB, UTF-8)
    ExitCode    *int
    StartedAt   time.Time
    CompletedAt *time.Time
    Error       string
}

type ExecutionStatus string
const (
    ExecutionStatusPending    ExecutionStatus = "pending"
    ExecutionStatusRunning    ExecutionStatus = "running"
    ExecutionStatusSuccess    ExecutionStatus = "success"
    ExecutionStatusFailed     ExecutionStatus = "failed"
    ExecutionStatusSkipped    ExecutionStatus = "skipped"
)
```

### DebugTask
```go
type DebugTask struct {
    ID          string
    ClusterID   string
    AgentID     string    // Specific agent to execute on
    Command     string
    Status      ExecutionStatus
    Output      string
    ExitCode    *int
    CreatedAt   time.Time
    ExecutedAt  *time.Time
    Error       string
}

// Lifecycle: Pending → Success/Failed → Deleted after 24h
// Pending tasks timeout after 1 hour
```

## gRPC API

### Service Definition

```protobuf
syntax = "proto3";

package maestro.agent.v1;

service AgentService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc PollTasks(PollTasksRequest) returns (PollTasksResponse);
  rpc ReportTaskExecution(ReportTaskExecutionRequest) returns (ReportTaskExecutionResponse);
  rpc PollDebugTasks(PollDebugTasksRequest) returns (PollDebugTasksResponse);
  rpc ReportDebugTaskExecution(ReportDebugTaskExecutionRequest) returns (ReportDebugTaskExecutionResponse);
}

message RegisterRequest {
  string agent_id = 1;   // Agent provides its own ID (computed from cluster_id + hostname)
  string cluster_id = 2;
  string hostname = 3;
}

message RegisterResponse {
  bool reset = 1;  // true if tasks were reset (re-registration)
}

message HeartbeatRequest {
  string agent_id = 1;
}

message HeartbeatResponse {
  bool acknowledged = 1;
}

message PollTasksRequest {
  string agent_id = 1;
}

message PollTasksResponse {
  repeated Task tasks = 1;
}

message Task {
  string id = 1;
  string name = 2;
  TaskType type = 3;
  bool blocking = 4;
  oneof config {
    ExecConfig exec = 10;
  }
}

message ExecConfig {
  string command = 1;
  int32 timeout_seconds = 2;
  string working_dir = 3;
}

message ReportTaskExecutionRequest {
  string agent_id = 1;
  string task_id = 2;
  ExecutionStatus status = 3;
  string output = 4;
  int32 exit_code = 5;
  string error = 6;
}

message ReportTaskExecutionResponse {
  bool acknowledged = 1;
}

message PollDebugTasksRequest {
  string agent_id = 1;
}

message PollDebugTasksResponse {
  repeated DebugTask debug_tasks = 1;
}

message DebugTask {
  string id = 1;
  string command = 2;
  int32 timeout_seconds = 3;
}

message ReportDebugTaskExecutionRequest {
  string agent_id = 1;
  string debug_task_id = 2;
  ExecutionStatus status = 3;
  string output = 4;
  int32 exit_code = 5;
  string error = 6;
}

message ReportDebugTaskExecutionResponse {
  bool acknowledged = 1;
}

enum TaskType {
  TASK_TYPE_UNSPECIFIED = 0;
  TASK_TYPE_EXEC = 1;
}

enum ExecutionStatus {
  EXECUTION_STATUS_UNSPECIFIED = 0;
  EXECUTION_STATUS_RUNNING = 1;
  EXECUTION_STATUS_SUCCESS = 2;
  EXECUTION_STATUS_FAILED = 3;
}
```

### gRPC Error Codes

**Register**:
- `OK`: Success
- `InvalidArgument`: Missing/invalid agent_id, cluster_id, or hostname
- `NotFound`: Cluster doesn't exist
- `Internal`: Database error

**Heartbeat**:
- `OK`: Success (always sets agent status to active)
- `NotFound`: Agent or cluster not found → agent should exit
- `Internal`: Database error

**PollTasks**:
- `OK`: Success (empty list if no tasks)
- `NotFound`: Agent not found
- `Internal`: Database error

**ReportTaskExecution**:
- `OK`: Success (acknowledged, uses upsert)
- `NotFound`: Task or agent not found
- `InvalidArgument`: Invalid status or missing fields
- `Internal`: Database error

**PollDebugTasks / ReportDebugTaskExecution**:
- `OK`: Success
- `NotFound`: Agent or debug task not found
- `Internal`: Database error

### gRPC Connection Management

**Agent Side**:
- Single persistent connection per agent
- Keepalive every 10 seconds
- Automatic reconnection with exponential backoff (max 5 attempts)
- Max message size: 10MB

```go
grpc.WithKeepaliveParams(keepalive.ClientParameters{
    Time:                10 * time.Second,
    Timeout:             3 * time.Second,
    PermitWithoutStream: true,
})
```

## REST API

### Clusters

**Create Cluster**:
```
POST /api/v1/clusters
Content-Type: application/json

Request:
{
  "name": "Production Cluster",
  "description": "Main production environment"
}

Response: 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Production Cluster",
  "description": "Main production environment",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**List Clusters**:
```
GET /api/v1/clusters?limit=100&offset=0

Response: 200 OK
{
  "data": [
    {
      "id": "...",
      "name": "Production Cluster",
      "description": "...",
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "pagination": {
    "limit": 100,
    "offset": 0,
    "total": 5,
    "has_more": false
  }
}
```

**Get Cluster**:
```
GET /api/v1/clusters/{id}

Response: 200 OK
{
  "id": "...",
  "name": "Production Cluster",
  "description": "...",
  "created_at": "...",
  "updated_at": "..."
}
```

**Delete Cluster**:
```
DELETE /api/v1/clusters/{id}

Response: 204 No Content

Note: Cascades to all agents, tasks, and executions
```

### Tasks

**Create Task** (always appends to end):
```
POST /api/v1/clusters/{clusterId}/tasks
Content-Type: application/json

Request:
{
  "name": "Install dependencies",
  "type": "exec",
  "blocking": true,
  "config": {
    "command": "apt-get update && apt-get install -y curl",
    "timeout_seconds": 1800,  // Optional, default: 1800 (30min)
    "working_dir": ""         // Optional, default: "" (agent's CWD)
  }
}

Response: 201 Created
{
  "id": "...",
  "cluster_id": "...",
  "name": "Install dependencies",
  "type": "exec",
  "order": 5,  // Auto-assigned (MAX(order) + 1)
  "blocking": true,
  "config": {
    "command": "apt-get update && apt-get install -y curl",
    "timeout_seconds": 1800,
    "working_dir": ""
  },
  "created_at": "...",
  "updated_at": "..."
}
```

**List Tasks**:
```
GET /api/v1/clusters/{clusterId}/tasks?limit=100&offset=0

Response: 200 OK
{
  "data": [
    {
      "id": "...",
      "name": "Install dependencies",
      "type": "exec",
      "order": 1,
      "blocking": true,
      "config": {...},
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "pagination": {...}
}
```

**Get Task**:
```
GET /api/v1/tasks/{id}

Response: 200 OK
{
  "id": "...",
  "cluster_id": "...",
  "name": "Install dependencies",
  "type": "exec",
  "order": 1,
  "blocking": true,
  "config": {...},
  "created_at": "...",
  "updated_at": "..."
}
```

**Update Task**:
```
PUT /api/v1/tasks/{id}
Content-Type: application/json

Request:
{
  "name": "Install dependencies v2",
  "blocking": false,
  "config": {
    "command": "apt-get install -y curl wget",
    "timeout_seconds": 3600
  }
}

Response: 200 OK
{
  "id": "...",
  "cluster_id": "...",
  "name": "Install dependencies v2",
  "type": "exec",
  "order": 1,  // Order unchanged
  "blocking": false,
  "config": {...},
  "created_at": "...",
  "updated_at": "2024-01-02T00:00:00Z"  // Updated
}

Note: Changes only affect future executions, not in-flight tasks
Note: Concurrent updates use last-write-wins (no optimistic locking)
```

**Delete Task** (soft delete + reorder):
```
DELETE /api/v1/tasks/{id}

Response: 204 No Content

Behavior:
- Sets deleted_at = NOW()
- Reorders remaining tasks to fill gap
- Example: [T1, T2, T3, T4] → delete T2 → [T1, T3→order2, T4→order3]
```

**Reorder Tasks**:
```
PUT /api/v1/clusters/{clusterId}/tasks/reorder
Content-Type: application/json

Request:
{
  "task_ids": ["id3", "id1", "id4", "id2"]
}

Response: 204 No Content

Behavior:
- Array position = new order
- id3 becomes order 1, id1 becomes order 2, etc.
- Must include ALL active tasks for the cluster
```

### Agents

**List Agents**:
```
GET /api/v1/clusters/{clusterId}/agents?limit=100&offset=0&status=active

Query params:
- limit: Page size (default: 100, max: 1000)
- offset: Page offset (default: 0)
- status: Filter by status (optional: active, inactive)
- sort: Sort field (default: registered_at)
- order: Sort order (default: desc)

Response: 200 OK
{
  "data": [
    {
      "id": "...",
      "cluster_id": "...",
      "hostname": "worker-01",
      "status": "active",
      "last_heartbeat": "2024-01-01T00:05:00Z",
      "registered_at": "2024-01-01T00:00:00Z",
      "last_reset_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "limit": 100,
    "offset": 0,
    "total": 9000,
    "has_more": true
  }
}
```

**Get Agent**:
```
GET /api/v1/agents/{id}

Response: 200 OK
{
  "id": "...",
  "cluster_id": "...",
  "hostname": "worker-01",
  "status": "active",
  "last_heartbeat": "2024-01-01T00:05:00Z",
  "registered_at": "2024-01-01T00:00:00Z",
  "last_reset_at": "2024-01-01T00:00:00Z"
}
```

**Delete Agent**:
```
DELETE /api/v1/agents/{id}

Response: 204 No Content

Note: Deletes agent and all its executions
```

### Monitoring

**Cluster Status**:
```
GET /api/v1/clusters/{clusterId}/status

Response: 200 OK
{
  "cluster_id": "...",
  "total_agents": 100,
  "active_agents": 95,
  "inactive_agents": 5,
  "total_tasks": 10,
  "agents_with_all_tasks_complete": 90,
  "agents_with_pending_tasks": 5,
  "agents_with_failed_tasks": 5,
  "completion_percentage": 90.0
}
```

**List Executions**:
```
GET /api/v1/clusters/{clusterId}/executions?limit=100&offset=0&status=failed

Query params:
- limit, offset: Pagination
- status: Filter by status (optional)
- agent_id: Filter by agent (optional)
- task_id: Filter by task (optional)

Response: 200 OK
{
  "data": [
    {
      "id": "...",
      "task_id": "...",
      "agent_id": "...",
      "status": "success",
      "output": "...",
      "exit_code": 0,
      "started_at": "...",
      "completed_at": "...",
      "error": null
    }
  ],
  "pagination": {...}
}
```

**Agent Execution History**:
```
GET /api/v1/agents/{id}/executions?limit=100&offset=0

Response: 200 OK (same format as above)
```

**Task Execution History** (across all agents):
```
GET /api/v1/tasks/{id}/executions?limit=100&offset=0

Response: 200 OK (same format as above)
```

### Debug

**Create Debug Task**:
```
POST /api/v1/agents/{agentId}/debug
Content-Type: application/json

Request:
{
  "command": "ps aux | grep nginx",
  "timeout_seconds": 30  // Optional, default: 300 (5min)
}

Response: 201 Created
{
  "id": "...",
  "agent_id": "...",
  "command": "ps aux | grep nginx",
  "status": "pending",
  "created_at": "2024-01-01T00:00:00Z"
}

Note: Pending debug tasks timeout after 1 hour if not executed
```

**List Debug Tasks**:
```
GET /api/v1/agents/{agentId}/debug?limit=100&offset=0

Response: 200 OK
{
  "data": [
    {
      "id": "...",
      "agent_id": "...",
      "command": "ps aux",
      "status": "success",
      "output": "USER  PID ...",
      "exit_code": 0,
      "created_at": "...",
      "executed_at": "..."
    }
  ],
  "pagination": {...}
}

Note: Completed debug tasks are deleted after 24 hours
```

## Core Components

### 1. Agent Registration

**When agent calls Register(agent_id, cluster_id, hostname)**:

```go
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
    // Validate UUID format
    if _, err := uuid.Parse(req.AgentID); err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
    }

    // Validate cluster exists
    if _, err := s.db.GetCluster(ctx, req.ClusterID); err != nil {
        return nil, status.Error(codes.NotFound, "cluster not found")
    }

    // Check if agent already exists
    agent, err := s.db.GetAgent(ctx, req.AgentID)

    if err == ErrNotFound {
        // New agent - create
        agent = &models.Agent{
            ID:            req.AgentID,
            ClusterID:     req.ClusterID,
            Hostname:      req.Hostname,
            Status:        AgentStatusActive,
            LastHeartbeat: time.Now(),
            RegisteredAt:  time.Now(),
            LastResetAt:   time.Now(),
        }
        s.db.CreateAgent(ctx, agent)
        return &RegisterResponse{Reset: false}, nil
    }

    // Agent re-registering - reset all tasks
    s.db.DeleteAllExecutionsForAgent(ctx, req.AgentID)
    s.db.UpdateAgent(ctx, req.AgentID, &AgentUpdate{
        Status:        AgentStatusActive,
        LastHeartbeat: time.Now(),
        LastResetAt:   time.Now(),
    })

    return &RegisterResponse{Reset: true}, nil
}
```

### 2. Heartbeat Handler

**Always reactivates agent on successful heartbeat**:

```go
func (s *Service) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
    agent, err := s.db.GetAgent(ctx, req.AgentID)
    if err != nil {
        // Agent or cluster not found → agent should exit
        return nil, status.Error(codes.NotFound, "agent not found")
    }

    // Update heartbeat and always set to active
    s.db.UpdateAgent(ctx, req.AgentID, &AgentUpdate{
        LastHeartbeat: time.Now(),
        Status:        AgentStatusActive,  // Reactivate if was inactive
    })

    return &HeartbeatResponse{Acknowledged: true}, nil
}
```

**Network Partition Recovery**:
- Agent misses 3 heartbeats → marked inactive by health monitor
- Network recovers → next heartbeat succeeds → agent reactivated automatically
- No re-registration needed for network partitions

### 3. Task Scheduler

**Algorithm**:
```
When agent polls for tasks:
1. Get agent's cluster_id
2. Get all tasks for cluster WHERE deleted_at IS NULL ORDER BY "order" ASC
3. Get ALL executions for this agent (success, failed, running, skipped)
4. Build pending task list:
   For each task in order:
     a. Check execution status:
        - If status = 'success' → skip (already done)
        - If status = 'running' → skip (currently executing)
        - If status = 'skipped' → skip (blocked previously)
     b. Check blocking dependencies:
        - If ANY previous blocking task has status = 'failed' → mark this and all remaining as skipped, break
     c. If this task has status = 'failed' AND task is non-blocking → skip this task, continue
     d. Otherwise → add to pending list
5. Return pending list
```

### 4. Health Monitor

```go
type HealthMonitor struct {
    db db.DB
    interval time.Duration  // 1 minute
    missedThreshold int     // 3
}

func (m *HealthMonitor) checkHealth(ctx context.Context) {
    // Find agents with last_heartbeat older than threshold
    threshold := time.Now().Add(-m.interval * time.Duration(m.missedThreshold))
    disconnectedAgents := m.db.FindAgentsWithHeartbeatBefore(ctx, threshold)

    for _, agent := range disconnectedAgents {
        // Mark agent as inactive
        m.db.UpdateAgentStatus(ctx, agent.ID, models.AgentStatusInactive)

        // Mark running tasks as failed
        m.db.FailRunningTasksForAgent(ctx, agent.ID)
    }
}

// FailRunningTasksForAgent marks all running tasks as failed
func (d *DB) FailRunningTasksForAgent(ctx context.Context, agentID string) error {
    query := `
        UPDATE task_executions
        SET status = 'failed',
            completed_at = NOW(),
            error = 'Agent became inactive during execution'
        WHERE agent_id = $1 AND status = 'running'
    `
    _, err := d.db.ExecContext(ctx, query, agentID)
    return err
}
```

### 5. Cleaner Service

**Purpose**: Remove old execution history and debug tasks.

**Rules**:
1. **For deleted tasks**: Delete executions after 7 days
2. **For active tasks**: Keep last 10 executions per agent per task
3. **Debug tasks**: Delete completed after 24 hours, timeout pending after 1 hour

```go
func (c *Cleaner) cleanup(ctx context.Context) {
    // 1. Delete executions for deleted tasks older than retention period
    threshold := time.Now().AddDate(0, 0, -c.retentionDays)
    c.db.DeleteExecutionsForDeletedTasksOlderThan(ctx, threshold)

    // 2. For active tasks: keep only last N executions per agent per task
    c.db.DeleteOldExecutionsKeepLastN(ctx, c.maxExecutionsPerAgentTask)

    // 3. Delete completed debug tasks older than 24 hours
    c.db.DeleteCompletedDebugTasksOlderThan(ctx, 24 * time.Hour)

    // 4. Timeout pending debug tasks older than 1 hour
    c.db.TimeoutPendingDebugTasks(ctx, 1 * time.Hour)
}

// Optimized cleanup using window functions
func (d *DB) DeleteOldExecutionsKeepLastN(ctx context.Context, keepN int) error {
    query := `
        WITH ranked_executions AS (
            SELECT
                te.id,
                ROW_NUMBER() OVER (
                    PARTITION BY te.agent_id, te.task_id
                    ORDER BY te.started_at DESC
                ) as row_num
            FROM task_executions te
            INNER JOIN tasks t ON te.task_id = t.id
            WHERE t.deleted_at IS NULL
        )
        DELETE FROM task_executions
        WHERE id IN (
            SELECT id FROM ranked_executions WHERE row_num > $1
        )
    `
    _, err := d.db.ExecContext(ctx, query, keepN)
    return err
}
```

### 6. Agent Behavior

**Startup**:
```go
func main() {
    // Get config
    cfg := loadConfig()

    // Generate agent ID
    hostname := cfg.Hostname
    if hostname == "auto" {
        hostname, _ = os.Hostname()
    }
    agentID := GenerateAgentID(cfg.ClusterID, hostname)

    // Connect to service
    conn := connectWithRetry(cfg.ServiceHost, cfg.ServicePort)
    client := agentv1.NewAgentServiceClient(conn)

    // Register
    resp := client.Register(ctx, &RegisterRequest{
        AgentID:   agentID,
        ClusterID: cfg.ClusterID,
        Hostname:  hostname,
    })

    if resp.Reset {
        slog.Warn("Re-registration detected, all tasks will be re-executed")
    }

    // Start agent
    agent := NewAgent(client, agentID, cfg)
    agent.Start(ctx)
}
```

**Main Loop**:
```go
func (a *Agent) Start(ctx context.Context) {
    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    // Start goroutines
    go a.heartbeatLoop(ctx)
    go a.taskLoop(ctx)
    go a.debugLoop(ctx)

    // Wait for shutdown signal
    <-sigCh
    slog.Info("Shutdown signal received, stopping new work...")
    cancel()

    // Grace period for current task to finish
    time.Sleep(30 * time.Second)
    slog.Info("Agent shutdown complete")
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
    ticker := time.NewTicker(a.heartbeatInterval)
    for {
        select {
        case <-ticker.C:
            _, err := a.client.Heartbeat(ctx, &HeartbeatRequest{
                AgentID: a.agentID,
            })
            if err != nil {
                if grpc.Code(err) == codes.NotFound {
                    slog.Error("Cluster deleted, exiting agent")
                    os.Exit(1)
                }
                slog.Warn("Heartbeat failed", "error", err)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (a *Agent) taskLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            slog.Info("Task loop stopping")
            return
        default:
            // Poll for tasks
            resp := a.client.PollTasks(ctx, &PollTasksRequest{
                AgentID: a.agentID,
            })

            if len(resp.Tasks) == 0 {
                time.Sleep(a.taskPollInterval)
                continue
            }

            // Execute tasks sequentially
            for _, task := range resp.Tasks {
                if ctx.Err() != nil {
                    return  // Stop on shutdown
                }
                a.executeTask(ctx, task)
            }
            // Immediately poll again (no sleep)
        }
    }
}

func (a *Agent) executeTask(ctx context.Context, task *Task) {
    // Validate timeout
    timeout := time.Duration(task.Config.TimeoutSeconds) * time.Second
    if timeout == 0 {
        timeout = 30 * time.Minute  // Default
    }
    if timeout > 24 * time.Hour {
        timeout = 24 * time.Hour  // Max
    }

    // Report running
    a.reportExecutionWithRetry(task.ID, ExecutionStatusRunning, "", 0, "")

    // Setup working directory
    workDir := task.Config.WorkingDir
    if workDir == "" {
        workDir, _ = os.Getwd()
    }
    if _, err := os.Stat(workDir); os.IsNotExist(err) {
        a.reportExecutionWithRetry(task.ID, ExecutionStatusFailed, "", -1,
            fmt.Sprintf("Working directory does not exist: %s", workDir))
        return
    }

    // Execute with timeout
    execCtx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    cmd := exec.CommandContext(execCtx, "bash", "-c", task.Config.Command)
    cmd.Dir = workDir
    output, err := cmd.CombinedOutput()

    // Sanitize and truncate output
    outputStr := sanitizeOutput(output)
    outputStr = truncateOutput(outputStr)

    if execCtx.Err() == context.DeadlineExceeded {
        // Timeout
        a.reportExecutionWithRetry(task.ID, ExecutionStatusFailed,
            outputStr, -1, "Task timeout exceeded")
    } else if err != nil {
        // Command failed
        exitCode := cmd.ProcessState.ExitCode()
        a.reportExecutionWithRetry(task.ID, ExecutionStatusFailed,
            outputStr, exitCode, err.Error())
    } else {
        // Success
        exitCode := cmd.ProcessState.ExitCode()
        a.reportExecutionWithRetry(task.ID, ExecutionStatusSuccess,
            outputStr, exitCode, "")
    }
}

// Sanitize output to valid UTF-8
func sanitizeOutput(output []byte) string {
    if utf8.Valid(output) {
        return string(output)
    }
    return strings.ToValidUTF8(string(output), "�")
}

// Truncate to last 1MB
func truncateOutput(output string) string {
    const MaxOutputSize = 1024 * 1024
    if len(output) <= MaxOutputSize {
        return output
    }
    truncated := output[len(output)-MaxOutputSize:]
    return "... (output truncated) ...\n" + truncated
}

func (a *Agent) reportExecutionWithRetry(taskID string, status ExecutionStatus,
    output string, exitCode int, errMsg string) {

    req := &ReportTaskExecutionRequest{
        AgentID:  a.agentID,
        TaskID:   taskID,
        Status:   status,
        Output:   output,
        ExitCode: exitCode,
        Error:    errMsg,
    }

    backoff := time.Second
    for attempt := 1; attempt <= 3; attempt++ {
        _, err := a.client.ReportTaskExecution(context.Background(), req)
        if err == nil {
            return
        }

        if grpc.Code(err) == codes.NotFound || grpc.Code(err) == codes.InvalidArgument {
            slog.Error("Failed to report execution", "error", err)
            return
        }

        slog.Warn("Failed to report execution, retrying", "attempt", attempt, "error", err)
        time.Sleep(backoff)
        backoff *= 2
    }

    slog.Error("Failed to report execution after retries")
}
```

**Debug Loop**:
```go
func (a *Agent) debugLoop(ctx context.Context) {
    ticker := time.NewTicker(a.debugPollInterval)  // 5 seconds
    for {
        select {
        case <-ticker.C:
            resp := a.client.PollDebugTasks(ctx, &PollDebugTasksRequest{
                AgentID: a.agentID,
            })

            for _, debugTask := range resp.DebugTasks {
                a.executeDebugTask(ctx, debugTask)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (a *Agent) executeDebugTask(ctx context.Context, task *DebugTask) {
    // Similar to executeTask but for debug commands
    // ...
    a.client.ReportDebugTaskExecution(ctx, &ReportDebugTaskExecutionRequest{
        AgentID:     a.agentID,
        DebugTaskID: task.ID,
        Status:      status,
        Output:      output,
        ExitCode:    exitCode,
        Error:       errMsg,
    })
}
```

## Configuration

### Service Configuration

```go
type Config struct {
    // Database (REQUIRED)
    DBURL string `env:"MAESTRO_DB_URL,required"`

    // gRPC Server (optional)
    GRPCHost string `env:"MAESTRO_GRPC_HOST" envDefault:"0.0.0.0"`
    GRPCPort int    `env:"MAESTRO_GRPC_PORT" envDefault:"9090"`

    // REST API (optional)
    RESTHost string `env:"MAESTRO_REST_HOST" envDefault:"0.0.0.0"`
    RESTPort int    `env:"MAESTRO_REST_PORT" envDefault:"8080"`

    // Health Monitor (optional)
    HealthCheckInterval   time.Duration `env:"MAESTRO_HEALTH_CHECK_INTERVAL" envDefault:"1m"`
    HealthMissedThreshold int           `env:"MAESTRO_HEALTH_MISSED_THRESHOLD" envDefault:"3"`

    // Cleaner (optional)
    CleanerInterval          time.Duration `env:"MAESTRO_CLEANER_INTERVAL" envDefault:"1h"`
    RetentionDays            int           `env:"MAESTRO_RETENTION_DAYS" envDefault:"7"`
    MaxExecutionsPerTask     int           `env:"MAESTRO_MAX_EXECUTIONS_PER_TASK" envDefault:"10"`

    // Limits (optional)
    MaxOutputSize int `env:"MAESTRO_MAX_OUTPUT_SIZE" envDefault:"1048576"`

    // DB Pool (optional)
    DBMaxOpenConns     int           `env:"MAESTRO_DB_MAX_OPEN_CONNS" envDefault:"100"`
    DBMaxIdleConns     int           `env:"MAESTRO_DB_MAX_IDLE_CONNS" envDefault:"10"`
    DBConnMaxLifetime  time.Duration `env:"MAESTRO_DB_CONN_MAX_LIFETIME" envDefault:"5m"`
    DBConnMaxIdleTime  time.Duration `env:"MAESTRO_DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`

    // Logging (optional)
    LogLevel  string `env:"MAESTRO_LOG_LEVEL" envDefault:"info"`
    LogFormat string `env:"MAESTRO_LOG_FORMAT" envDefault:"json"`
}
```

### Agent Configuration

```go
type Config struct {
    // Service (REQUIRED)
    ServiceHost string `env:"MAESTRO_SERVICE_HOST,required"`
    ServicePort int    `env:"MAESTRO_SERVICE_PORT" envDefault:"9090"`

    // Identity (REQUIRED)
    ClusterID string `env:"MAESTRO_CLUSTER_ID,required"`
    Hostname  string `env:"MAESTRO_HOSTNAME" envDefault:"auto"`

    // Intervals (optional)
    HeartbeatInterval time.Duration `env:"MAESTRO_HEARTBEAT_INTERVAL" envDefault:"1m"`
    TaskPollInterval  time.Duration `env:"MAESTRO_TASK_POLL_INTERVAL" envDefault:"1m"`
    DebugPollInterval time.Duration `env:"MAESTRO_DEBUG_POLL_INTERVAL" envDefault:"5s"`

    // Execution (optional)
    MaxOutputSize int `env:"MAESTRO_MAX_OUTPUT_SIZE" envDefault:"1048576"`

    // Logging (optional)
    LogLevel  string `env:"MAESTRO_LOG_LEVEL" envDefault:"info"`
    LogFormat string `env:"MAESTRO_LOG_FORMAT" envDefault:"json"`
}
```

## Database Schema

```sql
-- Clusters
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Agents
CREATE TABLE agents (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive')),
    last_heartbeat TIMESTAMP NOT NULL,
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_reset_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(cluster_id, hostname)
);

CREATE INDEX idx_agents_cluster ON agents(cluster_id);
CREATE INDEX idx_agents_heartbeat ON agents(last_heartbeat) WHERE status = 'active';

-- Tasks
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    "order" INT NOT NULL,
    blocking BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Partial unique index: only active tasks have unique order
CREATE UNIQUE INDEX idx_tasks_cluster_order_active
    ON tasks(cluster_id, "order")
    WHERE deleted_at IS NULL;

CREATE INDEX idx_tasks_cluster_order ON tasks(cluster_id, "order");
CREATE INDEX idx_tasks_deleted_at ON tasks(deleted_at) WHERE deleted_at IS NOT NULL;

-- Task Executions
CREATE TABLE task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    output TEXT,
    exit_code INT,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    error TEXT,
    CONSTRAINT unique_agent_task UNIQUE (agent_id, task_id)
);

CREATE INDEX idx_executions_agent_task ON task_executions(agent_id, task_id);
CREATE INDEX idx_executions_task ON task_executions(task_id);
CREATE INDEX idx_executions_status ON task_executions(status);

-- Debug Tasks
CREATE TABLE debug_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    output TEXT,
    exit_code INT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMP,
    error TEXT
);

CREATE INDEX idx_debug_tasks_agent_status ON debug_tasks(agent_id, status);
CREATE INDEX idx_debug_tasks_created ON debug_tasks(created_at);
```

## Database Migrations

**Tool**: golang-migrate

**Structure**:
```
migrations/
├── 000001_init_schema.up.sql
├── 000001_init_schema.down.sql
├── 000002_add_indexes.up.sql
└── 000002_add_indexes.down.sql
```

**Usage**:
```bash
# Install
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path migrations -database "$MAESTRO_DB_URL" up

# Rollback
migrate -path migrations -database "$MAESTRO_DB_URL" down 1
```

**In Code** (cmd/server/main.go):
```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(dbURL string) error {
    m, err := migrate.New("file://migrations", dbURL)
    if err != nil {
        return fmt.Errorf("migration init: %w", err)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migration up: %w", err)
    }
    return nil
}
```

## Service Graceful Shutdown

```go
func (s *Server) Start() error {
    ctx, cancel := context.WithCancel(context.Background())
    s.cancel = cancel

    // Start servers
    grpcServer := s.startGRPC()
    httpServer := s.startHTTP()

    // Start background services
    go s.healthMonitor.Start(ctx)
    go s.cleaner.Start(ctx)

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    <-sigCh

    slog.Info("Shutdown signal received")

    // Shutdown with 30s timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()

    // Stop accepting new connections
    grpcServer.GracefulStop()
    httpServer.Shutdown(shutdownCtx)

    // Stop background services
    cancel()

    // Close DB
    s.db.Close()

    slog.Info("Graceful shutdown complete")
    return nil
}
```

## Multiple Service Instances

**Stateless Design**: All state in PostgreSQL, service has no local state.

**Deployment**:
- Run N service instances
- Each instance runs all components (gRPC, REST, health monitor, cleaner)
- Share single PostgreSQL database
- Load balancer for REST API (optional)
- Agents connect to any instance (round-robin DNS)

**Database Connection Pooling**:
```
Per instance: max_open_conns = 100 / N instances
Example: 2 instances → 50 connections each
```

**Idempotent Operations**:
- Health monitor: Multiple instances mark same agents inactive (idempotent UPDATE)
- Cleaner: Multiple instances delete same old records (idempotent DELETE)

## Package Structure

```
maestro/
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── agent/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── grpc/
│   │   │   └── server.go
│   │   └── rest/
│   │       └── server.go
│   ├── models/
│   │   └── models.go
│   ├── db/
│   │   ├── db.go
│   │   └── postgres/
│   │       └── postgres.go
│   ├── engine/
│   │   └── scheduler.go
│   ├── monitor/
│   │   └── health.go
│   ├── cleaner/
│   │   └── cleaner.go
│   └── config/
│       └── config.go
├── proto/
│   └── agent/
│       └── v1/
│           └── agent.proto
├── api/
│   └── openapi.yaml
├── migrations/
│   ├── 000001_init_schema.up.sql
│   └── 000001_init_schema.down.sql
├── pkg/
│   └── client/
│       └── client.go
└── deployments/
    └── docker/
        ├── Dockerfile.server
        └── Dockerfile.agent
```

## Implementation Phases

### Phase 1 - Core Foundation
- [ ] Project setup (go mod, proto compilation)
- [ ] Database migrations (golang-migrate)
- [ ] Data models
- [ ] PostgreSQL DB implementation
- [ ] gRPC server (Register, Heartbeat, PollTasks, ReportExecution)
- [ ] Task scheduler engine
- [ ] Health monitor
- [ ] Configuration management
- [ ] Basic agent implementation
- [ ] Graceful shutdown (service + agent)

### Phase 2 - REST API & Management
- [ ] REST API (complete with pagination)
- [ ] OpenAPI documentation
- [ ] Cleaner service
- [ ] Debug mode (gRPC + REST)
- [ ] Docker containers
- [ ] Multi-instance deployment support

### Phase 3 - Observability & Testing
- [ ] Metrics (Prometheus)
- [ ] Structured logging improvements
- [ ] Performance testing with 9k agents
- [ ] Load testing
- [ ] Integration tests
- [ ] E2E tests

### Phase 4 - UI & Polish
- [ ] Web UI
- [ ] CLI tool
- [ ] Documentation
- [ ] Deployment guides

## Key Design Decisions

1. **Agent computes own ID**: Flexibility to change algorithm without service changes
2. **Re-registration = Reset**: Fail-safe for crashes (all tasks re-executed)
3. **Heartbeat auto-reactivates**: Network partitions don't require re-registration
4. **Pull-based**: Agents poll for tasks (service doesn't push)
5. **Sequential execution**: Agent executes one task at a time
6. **Separate heartbeat from task polling**: Long tasks don't block heartbeat
7. **Blocking tasks**: Failed blocking task skips remaining tasks
8. **Active tasks kept forever**: Tasks are templates for new agents
9. **Soft delete with reordering**: Task deletion fills gaps in order
10. **Tasks always append**: New tasks get `MAX(order) + 1`
11. **Idempotent reporting**: Duplicate reports override with latest (upsert)
12. **Client-side timeout**: Agent enforces task timeout
13. **Output truncation**: Last 1MB kept, UTF-8 sanitized
14. **Stateless service**: Multiple instances supported from Phase 1
15. **Last-write-wins**: No optimistic locking for task updates
16. **Graceful shutdown**: 30-second grace period for in-flight work
17. **Debug tasks timeout**: Pending after 1h, deleted after 24h
18. **Partial unique index**: Order constraint only for active tasks
