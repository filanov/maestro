package dto

import (
	"fmt"
	"time"

	"github.com/filanov/maestro/internal/models"
)

type DebugTaskResponse struct {
	ID         string     `json:"id"`
	ClusterID  string     `json:"cluster_id"`
	AgentID    string     `json:"agent_id"`
	Command    string     `json:"command"`
	Status     string     `json:"status"`
	Output     string     `json:"output,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExecutedAt *time.Time `json:"executed_at,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type CreateDebugTaskRequest struct {
	ClusterID string `json:"cluster_id"`
	AgentID   string `json:"agent_id"`
	Command   string `json:"command"`
}

func (r *CreateDebugTaskRequest) Validate() error {
	if r.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if r.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if r.Command == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}

func DebugTaskToResponse(task *models.DebugTask) DebugTaskResponse {
	return DebugTaskResponse{
		ID:         task.ID,
		ClusterID:  task.ClusterID,
		AgentID:    task.AgentID,
		Command:    task.Command,
		Status:     string(task.Status),
		Output:     task.Output,
		ExitCode:   task.ExitCode,
		CreatedAt:  task.CreatedAt,
		ExecutedAt: task.ExecutedAt,
		Error:      task.Error,
	}
}

func DebugTasksToResponse(tasks []*models.DebugTask) []DebugTaskResponse {
	result := make([]DebugTaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = DebugTaskToResponse(task)
	}
	return result
}
