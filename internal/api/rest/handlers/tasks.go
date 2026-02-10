package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/filanov/maestro/internal/api/rest/dto"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

type TaskHandler struct {
	db db.DB
}

func NewTaskHandler(database db.DB) *TaskHandler {
	return &TaskHandler{db: database}
}

func (h *TaskHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster_id")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster_id query parameter is required")
		return
	}

	includeDeleted := r.URL.Query().Get("include_deleted") == "true"
	limit, offset := parsePagination(r)

	tasks, total, err := h.db.ListTasks(r.Context(), clusterID, includeDeleted, limit, offset)
	if err != nil {
		slog.Error("failed to list tasks", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	writeJSON(w, http.StatusOK, dto.PaginatedResponse{
		Data:   dto.TasksToResponse(tasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *TaskHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tasks, err := h.db.GetTasksForCluster(r.Context(), req.ClusterID)
	if err != nil {
		slog.Error("failed to get tasks for cluster", "error", err, "cluster_id", req.ClusterID)
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	nextOrder := len(tasks)

	task := &models.Task{
		ID:        uuid.New().String(),
		ClusterID: req.ClusterID,
		Name:      req.Name,
		Type:      models.TaskType(req.Type),
		Order:     nextOrder,
		Blocking:  req.Blocking,
		Config:    dto.TaskConfigFromDTO(req.Config),
	}

	if err := h.db.CreateTask(r.Context(), task); err != nil {
		slog.Error("failed to create task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	writeJSON(w, http.StatusCreated, dto.TaskToResponse(task))
}

func (h *TaskHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	task, err := h.db.GetTask(r.Context(), id)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	writeJSON(w, http.StatusOK, dto.TaskToResponse(task))
}

func (h *TaskHandler) HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req dto.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	update := &db.TaskUpdate{
		Name:     req.Name,
		Blocking: req.Blocking,
	}

	if req.Config != nil {
		config := dto.TaskConfigFromDTO(*req.Config)
		update.Config = &config
	}

	if err := h.db.UpdateTask(r.Context(), id, update); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		slog.Error("failed to update task", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}

	task, err := h.db.GetTask(r.Context(), id)
	if err != nil {
		slog.Error("failed to get updated task", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get updated task")
		return
	}

	writeJSON(w, http.StatusOK, dto.TaskToResponse(task))
}

func (h *TaskHandler) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := h.db.DeleteTask(r.Context(), id); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "task not found")
		return
	} else if err != nil {
		slog.Error("failed to delete task", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) HandleReorderTasks(w http.ResponseWriter, r *http.Request) {
	var req dto.ReorderTasksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.db.ReorderTasks(r.Context(), req.ClusterID, req.TaskIDs); err != nil {
		slog.Error("failed to reorder tasks", "error", err, "cluster_id", req.ClusterID)
		writeError(w, http.StatusInternalServerError, "failed to reorder tasks")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
