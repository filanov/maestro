package rest

import (
	"net/http"

	"github.com/filanov/maestro/internal/api/rest/middleware"
	"github.com/go-chi/chi/v5"
)

func (s *Server) setupRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.RequestID)
	r.Use(middleware.CORS)

	r.Get("/health", s.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/clusters", func(r chi.Router) {
			r.Get("/", s.handleListClusters)
			r.Post("/", s.handleCreateCluster)
			r.Get("/{id}", s.handleGetCluster)
			r.Delete("/{id}", s.handleDeleteCluster)

			r.Route("/{clusterId}/agents", func(r chi.Router) {
				r.Get("/", s.handleListAgentsByCluster)
			})
		})

		r.Route("/agents", func(r chi.Router) {
			r.Get("/{id}", s.handleGetAgent)
			r.Delete("/{id}", s.handleDeleteAgent)
			r.Get("/{id}/executions", s.handleListAgentExecutions)
			r.Get("/{agentId}/debug-tasks", s.handleListDebugTasksByAgent)
		})

		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", s.handleListTasks)
			r.Post("/", s.handleCreateTask)
			r.Get("/{id}", s.handleGetTask)
			r.Put("/{id}", s.handleUpdateTask)
			r.Delete("/{id}", s.handleDeleteTask)
			r.Post("/reorder", s.handleReorderTasks)
		})

		r.Route("/executions", func(r chi.Router) {
			r.Get("/", s.handleListExecutions)
			r.Get("/{id}", s.handleGetExecution)
		})

		r.Route("/debug-tasks", func(r chi.Router) {
			r.Post("/", s.handleCreateDebugTask)
			r.Get("/{id}", s.handleGetDebugTask)
		})
	})

	return r
}
