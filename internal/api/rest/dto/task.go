package dto

import (
	"fmt"
	"time"

	"github.com/filanov/maestro/internal/models"
)

type TaskResponse struct {
	ID        string        `json:"id"`
	ClusterID string        `json:"cluster_id"`
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Order     int           `json:"order"`
	Blocking  bool          `json:"blocking"`
	Config    TaskConfigDTO `json:"config"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	DeletedAt *time.Time    `json:"deleted_at,omitempty"`
}

type TaskConfigDTO struct {
	Command    string `json:"command"`
	Timeout    int    `json:"timeout_seconds"`
	WorkingDir string `json:"working_dir"`
}

type CreateTaskRequest struct {
	ClusterID string        `json:"cluster_id"`
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Blocking  bool          `json:"blocking"`
	Config    TaskConfigDTO `json:"config"`
}

func (r *CreateTaskRequest) Validate() error {
	if r.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Name) > 255 {
		return fmt.Errorf("name must be less than 255 characters")
	}
	if r.Type == "" {
		return fmt.Errorf("type is required")
	}
	if r.Type != string(models.TaskTypeExec) {
		return fmt.Errorf("type must be 'exec'")
	}
	if r.Config.Command == "" {
		return fmt.Errorf("config.command is required")
	}
	if r.Config.Timeout <= 0 {
		return fmt.Errorf("config.timeout_seconds must be positive")
	}
	return nil
}

type UpdateTaskRequest struct {
	Name     *string        `json:"name,omitempty"`
	Blocking *bool          `json:"blocking,omitempty"`
	Config   *TaskConfigDTO `json:"config,omitempty"`
}

func (r *UpdateTaskRequest) Validate() error {
	if r.Name != nil && *r.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if r.Name != nil && len(*r.Name) > 255 {
		return fmt.Errorf("name must be less than 255 characters")
	}
	if r.Config != nil {
		if r.Config.Command == "" {
			return fmt.Errorf("config.command cannot be empty")
		}
		if r.Config.Timeout <= 0 {
			return fmt.Errorf("config.timeout_seconds must be positive")
		}
	}
	return nil
}

type ReorderTasksRequest struct {
	ClusterID string   `json:"cluster_id"`
	TaskIDs   []string `json:"task_ids"`
}

func (r *ReorderTasksRequest) Validate() error {
	if r.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if len(r.TaskIDs) == 0 {
		return fmt.Errorf("task_ids cannot be empty")
	}
	return nil
}

func TaskToResponse(task *models.Task) TaskResponse {
	return TaskResponse{
		ID:        task.ID,
		ClusterID: task.ClusterID,
		Name:      task.Name,
		Type:      string(task.Type),
		Order:     task.Order,
		Blocking:  task.Blocking,
		Config: TaskConfigDTO{
			Command:    task.Config.Command,
			Timeout:    int(task.Config.Timeout.Seconds()),
			WorkingDir: task.Config.WorkingDir,
		},
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
		DeletedAt: task.DeletedAt,
	}
}

func TasksToResponse(tasks []*models.Task) []TaskResponse {
	result := make([]TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = TaskToResponse(task)
	}
	return result
}

func TaskConfigFromDTO(dto TaskConfigDTO) models.TaskConfig {
	return models.TaskConfig{
		Command:    dto.Command,
		Timeout:    time.Duration(dto.Timeout) * time.Second,
		WorkingDir: dto.WorkingDir,
	}
}
