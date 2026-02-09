package db

import (
	"context"
	"errors"
	"time"

	"github.com/filanov/maestro/internal/models"
)

var ErrNotFound = errors.New("not found")

type DB interface {
	Close() error

	CreateCluster(ctx context.Context, cluster *models.Cluster) error
	GetCluster(ctx context.Context, id string) (*models.Cluster, error)
	ListClusters(ctx context.Context, limit, offset int) ([]*models.Cluster, int, error)
	DeleteCluster(ctx context.Context, id string) error

	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgent(ctx context.Context, id string) (*models.Agent, error)
	UpdateAgent(ctx context.Context, id string, update *AgentUpdate) error
	ListAgents(ctx context.Context, clusterID string, status *models.AgentStatus, limit, offset int) ([]*models.Agent, int, error)
	DeleteAgent(ctx context.Context, id string) error
	FindAgentsWithHeartbeatBefore(ctx context.Context, threshold time.Time) ([]*models.Agent, error)
	UpdateAgentStatus(ctx context.Context, id string, status models.AgentStatus) error

	CreateTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, id string, update *TaskUpdate) error
	DeleteTask(ctx context.Context, id string) error
	ListTasks(ctx context.Context, clusterID string, includeDeleted bool, limit, offset int) ([]*models.Task, int, error)
	GetTasksForCluster(ctx context.Context, clusterID string) ([]*models.Task, error)
	ReorderTasks(ctx context.Context, clusterID string, taskIDs []string) error

	CreateExecution(ctx context.Context, execution *models.TaskExecution) error
	UpsertExecution(ctx context.Context, execution *models.TaskExecution) error
	GetExecution(ctx context.Context, id string) (*models.TaskExecution, error)
	ListExecutions(ctx context.Context, filters ExecutionFilters, limit, offset int) ([]*models.TaskExecution, int, error)
	GetExecutionsForAgent(ctx context.Context, agentID string) ([]*models.TaskExecution, error)
	DeleteAllExecutionsForAgent(ctx context.Context, agentID string) error
	FailRunningTasksForAgent(ctx context.Context, agentID string) error

	CreateDebugTask(ctx context.Context, task *models.DebugTask) error
	GetDebugTask(ctx context.Context, id string) (*models.DebugTask, error)
	ListDebugTasks(ctx context.Context, agentID string, limit, offset int) ([]*models.DebugTask, int, error)
	GetPendingDebugTasksForAgent(ctx context.Context, agentID string) ([]*models.DebugTask, error)
	UpdateDebugTaskExecution(ctx context.Context, id string, status models.ExecutionStatus, output string, exitCode *int, error string) error
	DeleteCompletedDebugTasksOlderThan(ctx context.Context, threshold time.Time) error
	TimeoutPendingDebugTasks(ctx context.Context, threshold time.Time) error

	DeleteExecutionsForDeletedTasksOlderThan(ctx context.Context, threshold time.Time) error
	DeleteOldExecutionsKeepLastN(ctx context.Context, keepN int) error
}

type AgentUpdate struct {
	Status        *models.AgentStatus
	LastHeartbeat *time.Time
	LastResetAt   *time.Time
}

type TaskUpdate struct {
	Name     *string
	Blocking *bool
	Config   *models.TaskConfig
}

type ExecutionFilters struct {
	ClusterID *string
	AgentID   *string
	TaskID    *string
	Status    *models.ExecutionStatus
}
