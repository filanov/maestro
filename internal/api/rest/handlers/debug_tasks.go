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

type DebugTaskHandler struct {
	db db.DB
}

func NewDebugTaskHandler(database db.DB) *DebugTaskHandler {
	return &DebugTaskHandler{db: database}
}

func (h *DebugTaskHandler) HandleCreateDebugTask(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDebugTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	debugTask := &models.DebugTask{
		ID:        uuid.New().String(),
		ClusterID: req.ClusterID,
		AgentID:   req.AgentID,
		Command:   req.Command,
		Status:    models.ExecutionStatusPending,
	}

	if err := h.db.CreateDebugTask(r.Context(), debugTask); err != nil {
		slog.Error("failed to create debug task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create debug task")
		return
	}

	writeJSON(w, http.StatusCreated, dto.DebugTaskToResponse(debugTask))
}

func (h *DebugTaskHandler) HandleGetDebugTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "debug task id is required")
		return
	}

	debugTask, err := h.db.GetDebugTask(r.Context(), id)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "debug task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get debug task", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get debug task")
		return
	}

	writeJSON(w, http.StatusOK, dto.DebugTaskToResponse(debugTask))
}

func (h *DebugTaskHandler) HandleListDebugTasksByAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	limit, offset := parsePagination(r)

	debugTasks, total, err := h.db.ListDebugTasks(r.Context(), agentID, limit, offset)
	if err != nil {
		slog.Error("failed to list debug tasks", "error", err, "agent_id", agentID)
		writeError(w, http.StatusInternalServerError, "failed to list debug tasks")
		return
	}

	writeJSON(w, http.StatusOK, dto.PaginatedResponse{
		Data:   dto.DebugTasksToResponse(debugTasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
