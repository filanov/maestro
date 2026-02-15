package rest

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/filanov/maestro/internal/api/rest/openapi"
	"github.com/filanov/maestro/internal/models"
)

// UUID conversions

func uuidToString(u openapi_types.UUID) string {
	return uuid.UUID(u).String()
}

func stringToUUID(s string) openapi_types.UUID {
	u, _ := uuid.Parse(s)
	return openapi_types.UUID(u)
}

// Cluster conversions

func modelToOpenAPICluster(m *models.Cluster) openapi.Cluster {
	var description *string
	if m.Description != "" {
		description = &m.Description
	}

	return openapi.Cluster{
		Id:          stringToUUID(m.ID),
		Name:        m.Name,
		Description: description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func modelsToOpenAPIClusters(models []*models.Cluster) []openapi.Cluster {
	result := make([]openapi.Cluster, len(models))
	for i, m := range models {
		result[i] = modelToOpenAPICluster(m)
	}
	return result
}

// Agent conversions

func modelToOpenAPIAgent(m *models.Agent) openapi.Agent {
	var status openapi.AgentStatus
	if m.Status == models.AgentStatusActive {
		status = openapi.AgentStatusActive
	} else {
		status = openapi.AgentStatusInactive
	}

	return openapi.Agent{
		Id:            stringToUUID(m.ID),
		ClusterId:     stringToUUID(m.ClusterID),
		Hostname:      m.Hostname,
		Status:        status,
		RegisteredAt:  m.RegisteredAt,
		LastHeartbeat: m.LastHeartbeat,
	}
}

func modelsToOpenAPIAgents(models []*models.Agent) []openapi.Agent {
	result := make([]openapi.Agent, len(models))
	for i, m := range models {
		result[i] = modelToOpenAPIAgent(m)
	}
	return result
}

// Task conversions

func modelToOpenAPITask(m *models.Task) openapi.Task {
	config := openapi.TaskConfig{
		Command:        m.Config.Command,
		TimeoutSeconds: ptr(int(m.Config.Timeout.Seconds())),
		WorkingDir:     ptr(m.Config.WorkingDir),
	}

	return openapi.Task{
		Id:        stringToUUID(m.ID),
		ClusterId: stringToUUID(m.ClusterID),
		Name:      m.Name,
		Type:      openapi.TaskType(m.Type),
		Config:    config,
		Blocking:  m.Blocking,
		Order:     m.Order,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		DeletedAt: m.DeletedAt,
	}
}

func modelsToOpenAPITasks(models []*models.Task) []openapi.Task {
	result := make([]openapi.Task, len(models))
	for i, m := range models {
		result[i] = modelToOpenAPITask(m)
	}
	return result
}

// TaskExecution conversions

func modelToOpenAPITaskExecution(m *models.TaskExecution) openapi.TaskExecution {
	return openapi.TaskExecution{
		Id:          stringToUUID(m.ID),
		AgentId:     stringToUUID(m.AgentID),
		TaskId:      stringToUUID(m.TaskID),
		Status:      openapi.TaskExecutionStatus(m.Status),
		Output:      ptr(m.Output),
		ExitCode:    m.ExitCode,
		Error:       ptr(m.Error),
		StartedAt:   m.StartedAt,
		CompletedAt: m.CompletedAt,
	}
}

func modelsToOpenAPITaskExecutions(models []*models.TaskExecution) []openapi.TaskExecution {
	result := make([]openapi.TaskExecution, len(models))
	for i, m := range models {
		result[i] = modelToOpenAPITaskExecution(m)
	}
	return result
}

// DebugTask conversions

func modelToOpenAPIDebugTask(m *models.DebugTask) openapi.DebugTask {
	return openapi.DebugTask{
		Id:          stringToUUID(m.ID),
		AgentId:     stringToUUID(m.AgentID),
		Command:     m.Command,
		Status:      openapi.DebugTaskStatus(m.Status),
		Output:      ptr(m.Output),
		ExitCode:    m.ExitCode,
		Error:       ptr(m.Error),
		CreatedAt:   m.CreatedAt,
		CompletedAt: m.ExecutedAt, // Model field is ExecutedAt, API field is CompletedAt
	}
}

func modelsToOpenAPIDebugTasks(models []*models.DebugTask) []openapi.DebugTask {
	result := make([]openapi.DebugTask, len(models))
	for i, m := range models {
		result[i] = modelToOpenAPIDebugTask(m)
	}
	return result
}

// Helper to convert TaskConfig from OpenAPI to model
func taskConfigFromOpenAPI(config openapi.TaskConfig) models.TaskConfig {
	timeout := 300 * time.Second // default 5 minutes
	if config.TimeoutSeconds != nil {
		timeout = time.Duration(*config.TimeoutSeconds) * time.Second
	}

	workingDir := ""
	if config.WorkingDir != nil {
		workingDir = *config.WorkingDir
	}

	return models.TaskConfig{
		Command:    config.Command,
		Timeout:    timeout,
		WorkingDir: workingDir,
	}
}

// Helper to create pointer
func ptr[T any](v T) *T {
	return &v
}
