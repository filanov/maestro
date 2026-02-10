package handlers

import (
	"net/http"

	"github.com/filanov/maestro/internal/api/rest/dto"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dto.HealthResponse{
		Status: "ok",
	})
}
