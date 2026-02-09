package models

import (
	"time"

	"github.com/google/uuid"
)

type Cluster struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Agent struct {
	ID            string
	ClusterID     string
	Hostname      string
	Status        AgentStatus
	LastHeartbeat time.Time
	RegisteredAt  time.Time
	LastResetAt   time.Time
}

type AgentStatus string

const (
	AgentStatusActive   AgentStatus = "active"
	AgentStatusInactive AgentStatus = "inactive"
)

type Task struct {
	ID        string
	ClusterID string
	Name      string
	Type      TaskType
	Order     int
	Blocking  bool
	Config    TaskConfig
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type TaskType string

const (
	TaskTypeExec TaskType = "exec"
)

type TaskConfig struct {
	Command    string        `json:"command"`
	Timeout    time.Duration `json:"timeout_seconds"`
	WorkingDir string        `json:"working_dir"`
}

type TaskExecution struct {
	ID          string
	TaskID      string
	AgentID     string
	Status      ExecutionStatus
	Output      string
	ExitCode    *int
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

type ExecutionStatus string

const (
	ExecutionStatusPending ExecutionStatus = "pending"
	ExecutionStatusRunning ExecutionStatus = "running"
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusFailed  ExecutionStatus = "failed"
	ExecutionStatusSkipped ExecutionStatus = "skipped"
)

type DebugTask struct {
	ID         string
	ClusterID  string
	AgentID    string
	Command    string
	Status     ExecutionStatus
	Output     string
	ExitCode   *int
	CreatedAt  time.Time
	ExecutedAt *time.Time
	Error      string
}

func GenerateAgentID(clusterID, hostname string) string {
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	data := clusterID + ":" + hostname
	return uuid.NewSHA1(namespace, []byte(data)).String()
}
