package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/filanov/maestro/internal/api/rest/middleware"
	"github.com/filanov/maestro/internal/api/rest/openapi"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
)

type Server struct {
	db         db.DB
	scheduler  *engine.Scheduler
	httpServer *http.Server
}

func NewServer(database db.DB, scheduler *engine.Scheduler, host string, port int) *Server {
	s := &Server{
		db:        database,
		scheduler: scheduler,
	}

	router := chi.NewRouter()

	router.Use(middleware.Logging)
	router.Use(middleware.Recovery)
	router.Use(middleware.CORS)

	openapi.HandlerFromMux(s, router)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	slog.Info("starting REST server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("REST server failed: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down REST server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openapi.HealthResponse{
		Status: ptr("ok"),
	})
}

func (s *Server) ListClusters(w http.ResponseWriter, r *http.Request, params openapi.ListClustersParams) {
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	clusters, total, err := s.db.ListClusters(r.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list clusters", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list clusters")
		return
	}

	response := struct {
		Data   []openapi.Cluster `json:"data"`
		Total  int               `json:"total"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}{
		Data:   modelsToOpenAPIClusters(clusters),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req openapi.CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	cluster := &models.Cluster{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: description,
	}

	if err := s.db.CreateCluster(r.Context(), cluster); err != nil {
		slog.Error("failed to create cluster", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create cluster")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPICluster(cluster))
}

func (s *Server) GetCluster(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	clusterID := uuidToString(id)

	cluster, err := s.db.GetCluster(r.Context(), clusterID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}
	if err != nil {
		slog.Error("failed to get cluster", "error", err, "id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to get cluster")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPICluster(cluster))
}

func (s *Server) DeleteCluster(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	clusterID := uuidToString(id)

	if err := s.db.DeleteCluster(r.Context(), clusterID); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	} else if err != nil {
		slog.Error("failed to delete cluster", "error", err, "id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to delete cluster")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListAgentsByCluster(w http.ResponseWriter, r *http.Request, clusterId openapi_types.UUID, params openapi.ListAgentsByClusterParams) {
	clusterID := uuidToString(clusterId)
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	agents, total, err := s.db.ListAgents(r.Context(), clusterID, nil, limit, offset)
	if err != nil {
		slog.Error("failed to list agents", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	response := struct {
		Data   []openapi.Agent `json:"data"`
		Total  int             `json:"total"`
		Limit  int             `json:"limit"`
		Offset int             `json:"offset"`
	}{
		Data:   modelsToOpenAPIAgents(agents),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) GetAgent(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	agentID := uuidToString(id)

	agent, err := s.db.GetAgent(r.Context(), agentID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		slog.Error("failed to get agent", "error", err, "id", agentID)
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPIAgent(agent))
}

func (s *Server) DeleteAgent(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	agentID := uuidToString(id)

	if err := s.db.DeleteAgent(r.Context(), agentID); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	} else if err != nil {
		slog.Error("failed to delete agent", "error", err, "id", agentID)
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListAgentExecutions(w http.ResponseWriter, r *http.Request, id openapi.ID, params openapi.ListAgentExecutionsParams) {
	agentID := uuidToString(id)
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	filters := db.ExecutionFilters{
		AgentID: &agentID,
	}
	executions, total, err := s.db.ListExecutions(r.Context(), filters, limit, offset)
	if err != nil {
		slog.Error("failed to list agent executions", "error", err, "agent_id", agentID)
		writeError(w, http.StatusInternalServerError, "failed to list executions")
		return
	}

	response := struct {
		Data   []openapi.TaskExecution `json:"data"`
		Total  int                     `json:"total"`
		Limit  int                     `json:"limit"`
		Offset int                     `json:"offset"`
	}{
		Data:   modelsToOpenAPITaskExecutions(executions),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request, params openapi.ListTasksParams) {
	clusterID := uuidToString(params.ClusterId)
	includeDeleted := false
	if params.IncludeDeleted != nil {
		includeDeleted = *params.IncludeDeleted
	}
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	tasks, total, err := s.db.ListTasks(r.Context(), clusterID, includeDeleted, limit, offset)
	if err != nil {
		slog.Error("failed to list tasks", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	response := struct {
		Data   []openapi.Task `json:"data"`
		Total  int            `json:"total"`
		Limit  int            `json:"limit"`
		Offset int            `json:"offset"`
	}{
		Data:   modelsToOpenAPITasks(tasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req openapi.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Config.Command == "" {
		writeError(w, http.StatusBadRequest, "config.command is required")
		return
	}

	clusterID := uuidToString(req.ClusterId)
	tasks, err := s.db.GetTasksForCluster(r.Context(), clusterID)
	if err != nil {
		slog.Error("failed to get tasks for cluster", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	nextOrder := len(tasks)

	blocking := false
	if req.Blocking != nil {
		blocking = *req.Blocking
	}

	task := &models.Task{
		ID:        uuid.New().String(),
		ClusterID: clusterID,
		Name:      req.Name,
		Type:      models.TaskType(req.Type),
		Order:     nextOrder,
		Blocking:  blocking,
		Config:    taskConfigFromOpenAPI(req.Config),
	}

	if err := s.db.CreateTask(r.Context(), task); err != nil {
		slog.Error("failed to create task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPITask(task))
}

func (s *Server) GetTask(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	taskID := uuidToString(id)

	task, err := s.db.GetTask(r.Context(), taskID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "error", err, "id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPITask(task))
}

func (s *Server) UpdateTask(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	taskID := uuidToString(id)

	var req openapi.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	update := &db.TaskUpdate{
		Name:     req.Name,
		Blocking: req.Blocking,
	}

	if req.Config != nil {
		config := taskConfigFromOpenAPI(*req.Config)
		update.Config = &config
	}

	if err := s.db.UpdateTask(r.Context(), taskID, update); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		slog.Error("failed to update task", "error", err, "id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}

	if err := s.db.ResetExecutionsForTask(r.Context(), taskID); err != nil {
		slog.Warn("failed to reset executions after task update", "error", err, "id", taskID)
	} else {
		slog.Info("task executions reset after update", "task_id", taskID)
	}

	task, err := s.db.GetTask(r.Context(), taskID)
	if err != nil {
		slog.Error("failed to get updated task", "error", err, "id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to get updated task")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPITask(task))
}

func (s *Server) DeleteTask(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	taskID := uuidToString(id)

	if err := s.db.DeleteTask(r.Context(), taskID); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		slog.Error("failed to delete task", "error", err, "id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReorderTasks(w http.ResponseWriter, r *http.Request) {
	var req openapi.ReorderTasksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.TaskIds) == 0 {
		writeError(w, http.StatusBadRequest, "task_ids is required and cannot be empty")
		return
	}

	clusterID := uuidToString(req.ClusterId)
	tasks, err := s.db.GetTasksForCluster(r.Context(), clusterID)
	if err != nil {
		slog.Error("failed to get tasks for reorder", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to get tasks")
		return
	}

	oldOrderMap := make(map[string]int)
	for _, task := range tasks {
		oldOrderMap[task.ID] = task.Order
	}

	taskIDs := make([]string, len(req.TaskIds))
	for i, id := range req.TaskIds {
		taskIDs[i] = uuidToString(id)
	}

	if err := s.db.ReorderTasks(r.Context(), clusterID, taskIDs); err != nil {
		slog.Error("failed to reorder tasks", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to reorder tasks")
		return
	}

	var changedTasks []string
	for newPosition, taskID := range taskIDs {
		oldPosition, exists := oldOrderMap[taskID]
		if !exists {
			continue
		}
		if oldPosition != newPosition {
			changedTasks = append(changedTasks, taskID)
		}
	}

	for _, taskID := range changedTasks {
		if err := s.db.ResetExecutionsForTask(r.Context(), taskID); err != nil {
			slog.Warn("failed to reset executions after reorder", "error", err, "task_id", taskID)
		} else {
			slog.Info("task executions reset due to order change", "task_id", taskID)
		}
	}

	if len(changedTasks) > 0 {
		slog.Info("reorder completed with execution resets", "cluster_id", clusterID, "changed_tasks", len(changedTasks))
	} else {
		slog.Info("reorder completed with no position changes", "cluster_id", clusterID)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ResetTaskExecutions(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	taskID := uuidToString(id)

	if _, err := s.db.GetTask(r.Context(), taskID); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		slog.Error("failed to get task", "error", err, "id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to verify task")
		return
	}

	if err := s.db.ResetExecutionsForTask(r.Context(), taskID); err != nil {
		slog.Error("failed to reset task executions", "error", err, "task_id", taskID)
		writeError(w, http.StatusInternalServerError, "failed to reset task executions")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListExecutions(w http.ResponseWriter, r *http.Request, params openapi.ListExecutionsParams) {
	filters := db.ExecutionFilters{}

	if params.ClusterId != nil {
		clusterID := uuidToString(*params.ClusterId)
		filters.ClusterID = &clusterID
	}
	if params.AgentId != nil {
		agentID := uuidToString(*params.AgentId)
		filters.AgentID = &agentID
	}
	if params.TaskId != nil {
		taskID := uuidToString(*params.TaskId)
		filters.TaskID = &taskID
	}
	if params.Status != nil {
		execStatus := models.ExecutionStatus(*params.Status)
		filters.Status = &execStatus
	}

	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	executions, total, err := s.db.ListExecutions(r.Context(), filters, limit, offset)
	if err != nil {
		slog.Error("failed to list executions", "error", err, "filters", filters)
		writeError(w, http.StatusInternalServerError, "failed to list executions")
		return
	}

	response := struct {
		Data   []openapi.TaskExecution `json:"data"`
		Total  int                     `json:"total"`
		Limit  int                     `json:"limit"`
		Offset int                     `json:"offset"`
	}{
		Data:   modelsToOpenAPITaskExecutions(executions),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) GetExecution(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	executionID := uuidToString(id)

	execution, err := s.db.GetExecution(r.Context(), executionID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	if err != nil {
		slog.Error("failed to get execution", "error", err, "id", executionID)
		writeError(w, http.StatusInternalServerError, "failed to get execution")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPITaskExecution(execution))
}

func (s *Server) CreateDebugTask(w http.ResponseWriter, r *http.Request) {
	var req openapi.CreateDebugTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	agentID := uuidToString(req.AgentId)
	agent, err := s.db.GetAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	debugTask := &models.DebugTask{
		ID:        uuid.New().String(),
		ClusterID: agent.ClusterID,
		AgentID:   agentID,
		Command:   req.Command,
		Status:    models.ExecutionStatusPending,
	}

	if err := s.db.CreateDebugTask(r.Context(), debugTask); err != nil {
		slog.Error("failed to create debug task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create debug task")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPIDebugTask(debugTask))
}

func (s *Server) GetDebugTask(w http.ResponseWriter, r *http.Request, id openapi.ID) {
	debugTaskID := uuidToString(id)

	debugTask, err := s.db.GetDebugTask(r.Context(), debugTaskID)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "debug task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get debug task", "error", err, "id", debugTaskID)
		writeError(w, http.StatusInternalServerError, "failed to get debug task")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPIDebugTask(debugTask))
}

func (s *Server) ListDebugTasksByAgent(w http.ResponseWriter, r *http.Request, agentId openapi_types.UUID, params openapi.ListDebugTasksByAgentParams) {
	agentID := uuidToString(agentId)
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	debugTasks, total, err := s.db.ListDebugTasks(r.Context(), agentID, limit, offset)
	if err != nil {
		slog.Error("failed to list debug tasks", "error", err, "agent_id", agentID)
		writeError(w, http.StatusInternalServerError, "failed to list debug tasks")
		return
	}

	response := struct {
		Data   []openapi.DebugTask `json:"data"`
		Total  int                 `json:"total"`
		Limit  int                 `json:"limit"`
		Offset int                 `json:"offset"`
	}{
		Data:   modelsToOpenAPIDebugTasks(debugTasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) ListTemplates(w http.ResponseWriter, r *http.Request, params openapi.ListTemplatesParams) {
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	templates, total, err := s.db.ListTemplates(r.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list templates", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	response := struct {
		Data   []openapi.Template `json:"data"`
		Total  int                `json:"total"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
	}{
		Data:   modelsToOpenAPITemplates(templates),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req openapi.CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	template := &models.Template{
		Name: req.Name,
	}
	if req.Description != nil {
		template.Description = *req.Description
	}

	if err := s.db.CreateTemplate(r.Context(), template); err != nil {
		slog.Error("failed to create template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPITemplate(template))
}

func (s *Server) GetTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	template, err := s.db.GetTemplate(r.Context(), uuidToString(id))
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		slog.Error("failed to get template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get template")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPITemplate(template))
}

func (s *Server) UpdateTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var req openapi.UpdateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	update := &db.TemplateUpdate{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.db.UpdateTemplate(r.Context(), uuidToString(id), update); err != nil {
		slog.Error("failed to update template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

	template, err := s.db.GetTemplate(r.Context(), uuidToString(id))
	if err != nil {
		slog.Error("failed to get updated template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated template")
		return
	}

	writeJSON(w, http.StatusOK, modelToOpenAPITemplate(template))
}

func (s *Server) DeleteTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if err := s.db.DeleteTemplate(r.Context(), uuidToString(id)); err != nil {
		slog.Error("failed to delete template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListTemplateTasks(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, params openapi.ListTemplateTasksParams) {
	limit, offset := parsePaginationParams(params.Limit, params.Offset)

	tasks, total, err := s.db.ListTemplateTasks(r.Context(), uuidToString(id), limit, offset)
	if err != nil {
		slog.Error("failed to list template tasks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list template tasks")
		return
	}

	response := struct {
		Data   []openapi.TemplateTask `json:"data"`
		Total  int                    `json:"total"`
		Limit  int                    `json:"limit"`
		Offset int                    `json:"offset"`
	}{
		Data:   modelsToOpenAPITemplateTasks(tasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) CreateTemplateTask(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var req openapi.CreateTemplateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existingTasks, err := s.db.GetTemplateTasksForTemplate(r.Context(), uuidToString(id))
	if err != nil {
		slog.Error("failed to get existing template tasks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get existing template tasks")
		return
	}

	maxOrder := 0
	for _, task := range existingTasks {
		if task.Order > maxOrder {
			maxOrder = task.Order
		}
	}

	blocking := false
	if req.Blocking != nil {
		blocking = *req.Blocking
	}

	timeout := 300 * time.Second
	if req.Config.TimeoutSeconds != nil {
		timeout = time.Duration(*req.Config.TimeoutSeconds) * time.Second
	}

	task := &models.TemplateTask{
		TemplateID: uuidToString(id),
		Name:       req.Name,
		Type:       models.TaskType(req.Type),
		Order:      maxOrder + 1,
		Blocking:   blocking,
		Config: models.TaskConfig{
			Command:    req.Config.Command,
			Timeout:    timeout,
			WorkingDir: stringPtrToString(req.Config.WorkingDir),
		},
	}

	if err := s.db.CreateTemplateTask(r.Context(), task); err != nil {
		slog.Error("failed to create template task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create template task")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPITemplateTask(task))
}

func (s *Server) ImportTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var req openapi.ImportTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	templateTasks, err := s.db.GetTemplateTasksForTemplate(r.Context(), uuidToString(req.TemplateId))
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		slog.Error("failed to get template tasks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get template tasks")
		return
	}

	if err := s.db.ImportTemplateToCluster(r.Context(), uuidToString(id), uuidToString(req.TemplateId)); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster or template not found")
		return
	} else if err != nil {
		slog.Error("failed to import template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to import template")
		return
	}

	response := openapi.ImportTemplateResponse{
		Message:       "template imported successfully",
		TasksImported: len(templateTasks),
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) ExportTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var req openapi.ExportTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	template := &models.Template{
		Name: req.TemplateName,
	}
	if req.TemplateDescription != nil {
		template.Description = *req.TemplateDescription
	}

	var taskIDs []string
	if req.TaskIds != nil {
		taskIDs = make([]string, len(*req.TaskIds))
		for i, uuid := range *req.TaskIds {
			taskIDs[i] = uuidToString(uuid)
		}
	}

	if err := s.db.ExportClusterToTemplate(r.Context(), uuidToString(id), template, taskIDs); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	} else if err != nil {
		slog.Error("failed to export cluster to template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to export cluster to template")
		return
	}

	writeJSON(w, http.StatusCreated, modelToOpenAPITemplate(template))
}

func parsePaginationParams(limitPtr *int, offsetPtr *int) (limit, offset int) {
	limit = 50
	offset = 0

	if limitPtr != nil {
		if *limitPtr > 0 {
			limit = *limitPtr
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	if offsetPtr != nil {
		if *offsetPtr >= 0 {
			offset = *offsetPtr
		}
	}

	return limit, offset
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, openapi.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: ptr(message),
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
