package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/filanov/maestro/internal/api/rest/dto"
	"github.com/filanov/maestro/internal/db"
)

type AgentHandler struct {
	db db.DB
}

func NewAgentHandler(database db.DB) *AgentHandler {
	return &AgentHandler{db: database}
}

func (h *AgentHandler) HandleListAgentsByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterId")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster id is required")
		return
	}

	limit, offset := parsePagination(r)

	agents, total, err := h.db.ListAgents(r.Context(), clusterID, nil, limit, offset)
	if err != nil {
		slog.Error("failed to list agents", "error", err, "cluster_id", clusterID)
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	writeJSON(w, http.StatusOK, dto.PaginatedResponse{
		Data:   dto.AgentsToResponse(agents),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *AgentHandler) HandleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	agent, err := h.db.GetAgent(r.Context(), id)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		slog.Error("failed to get agent", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	writeJSON(w, http.StatusOK, dto.AgentToResponse(agent))
}

func (h *AgentHandler) HandleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if err := h.db.DeleteAgent(r.Context(), id); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	} else if err != nil {
		slog.Error("failed to delete agent", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AgentHandler) HandleListAgentExecutions(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	limit, offset := parsePagination(r)

	filters := db.ExecutionFilters{
		AgentID: &agentID,
	}
	executions, total, err := h.db.ListExecutions(r.Context(), filters, limit, offset)
	if err != nil {
		slog.Error("failed to list agent executions", "error", err, "agent_id", agentID)
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
