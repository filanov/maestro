package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/filanov/maestro/internal/api/rest/dto"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

type ClusterHandler struct {
	db db.DB
}

func NewClusterHandler(database db.DB) *ClusterHandler {
	return &ClusterHandler{db: database}
}

func (h *ClusterHandler) HandleListClusters(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	clusters, total, err := h.db.ListClusters(r.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list clusters", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list clusters")
		return
	}

	writeJSON(w, http.StatusOK, dto.PaginatedResponse{
		Data:   dto.ClustersToResponse(clusters),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *ClusterHandler) HandleCreateCluster(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cluster := &models.Cluster{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.db.CreateCluster(r.Context(), cluster); err != nil {
		slog.Error("failed to create cluster", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create cluster")
		return
	}

	writeJSON(w, http.StatusCreated, dto.ClusterToResponse(cluster))
}

func (h *ClusterHandler) HandleGetCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "cluster id is required")
		return
	}

	cluster, err := h.db.GetCluster(r.Context(), id)
	if err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}
	if err != nil {
		slog.Error("failed to get cluster", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get cluster")
		return
	}

	writeJSON(w, http.StatusOK, dto.ClusterToResponse(cluster))
}

func (h *ClusterHandler) HandleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "cluster id is required")
		return
	}

	if err := h.db.DeleteCluster(r.Context(), id); err == db.ErrNotFound {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	} else if err != nil {
		slog.Error("failed to delete cluster", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to delete cluster")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, dto.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
