package dto

import (
	"time"

	"github.com/filanov/maestro/internal/models"
)

type ExecutionResponse struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	AgentID     string     `json:"agent_id"`
	Status      string     `json:"status"`
	Output      string     `json:"output"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func ExecutionToResponse(exec *models.TaskExecution) ExecutionResponse {
	return ExecutionResponse{
		ID:          exec.ID,
		TaskID:      exec.TaskID,
		AgentID:     exec.AgentID,
		Status:      string(exec.Status),
		Output:      exec.Output,
		ExitCode:    exec.ExitCode,
		StartedAt:   exec.StartedAt,
		CompletedAt: exec.CompletedAt,
		Error:       exec.Error,
	}
}

func ExecutionsToResponse(execs []*models.TaskExecution) []ExecutionResponse {
	result := make([]ExecutionResponse, len(execs))
	for i, exec := range execs {
		result[i] = ExecutionToResponse(exec)
	}
	return result
}
