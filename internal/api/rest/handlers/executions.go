package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/filanov/maestro/internal/api/rest/dto"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

type ExecutionHandler struct {
	db db.DB
}

func NewExecutionHandler(database db.DB) *ExecutionHandler {
	return &ExecutionHandler{db: database}
}

func (h *ExecutionHandler) HandleListExecutions(w http.ResponseWriter, r *http.Request) {
	filters := db.ExecutionFilters{}

	if clusterID := r.URL.Query().Get("cluster_id"); clusterID != "" {
		filters.ClusterID = &clusterID
	}
	if agentID := r.URL.Query().Get("agent_id"); agentID != "" {
		filters.AgentID = &agentID
	}
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		filters.TaskID = &taskID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		execStatus := models.ExecutionStatus(status)
		filters.Status = &execStatus
	}

	limit, offset := parsePagination(r)

	executions, total, err := h.db.ListExecutions(r.Context(), filters, limit, offset)
	if err != nil {
		slog.Error("failed to list executions", "error", err, "filters", filters)
		writeError(w, http.StatusInternalServerError, "failed to list executions")
		return
	}

	writeJSON(w, http.StatusOK, dto.PaginatedResponse{
		Data:   dto.ExecutionsToResponse(executions),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *ExecutionHandler) HandleGetExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "execution id is required")
		return
	}

	execution, err := h.db.GetExecution(r.Context(), id)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	if err != nil {
		slog.Error("failed to get execution", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get execution")
		return
	}

	writeJSON(w, http.StatusOK, dto.ExecutionToResponse(execution))
}
