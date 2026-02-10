package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/filanov/maestro/internal/api/rest/handlers"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
)

type Server struct {
	db         db.DB
	scheduler  *engine.Scheduler
	httpServer *http.Server

	clusterHandler   *handlers.ClusterHandler
	healthHandler    *handlers.HealthHandler
	agentHandler     *handlers.AgentHandler
	taskHandler      *handlers.TaskHandler
	executionHandler *handlers.ExecutionHandler
	debugTaskHandler *handlers.DebugTaskHandler
}

func NewServer(database db.DB, scheduler *engine.Scheduler, host string, port int) *Server {
	s := &Server{
		db:               database,
		scheduler:        scheduler,
		clusterHandler:   handlers.NewClusterHandler(database),
		healthHandler:    handlers.NewHealthHandler(),
		agentHandler:     handlers.NewAgentHandler(database),
		taskHandler:      handlers.NewTaskHandler(database),
		executionHandler: handlers.NewExecutionHandler(database),
		debugTaskHandler: handlers.NewDebugTaskHandler(database),
	}

	router := s.setupRouter()

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

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	s.clusterHandler.HandleListClusters(w, r)
}

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	s.clusterHandler.HandleCreateCluster(w, r)
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	s.clusterHandler.HandleGetCluster(w, r)
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	s.clusterHandler.HandleDeleteCluster(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.healthHandler.HandleHealth(w, r)
}

func (s *Server) handleListAgentsByCluster(w http.ResponseWriter, r *http.Request) {
	s.agentHandler.HandleListAgentsByCluster(w, r)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	s.agentHandler.HandleGetAgent(w, r)
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	s.agentHandler.HandleDeleteAgent(w, r)
}

func (s *Server) handleListAgentExecutions(w http.ResponseWriter, r *http.Request) {
	s.agentHandler.HandleListAgentExecutions(w, r)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleListTasks(w, r)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleCreateTask(w, r)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleGetTask(w, r)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleUpdateTask(w, r)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleDeleteTask(w, r)
}

func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	s.taskHandler.HandleReorderTasks(w, r)
}

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	s.executionHandler.HandleListExecutions(w, r)
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	s.executionHandler.HandleGetExecution(w, r)
}

func (s *Server) handleCreateDebugTask(w http.ResponseWriter, r *http.Request) {
	s.debugTaskHandler.HandleCreateDebugTask(w, r)
}

func (s *Server) handleGetDebugTask(w http.ResponseWriter, r *http.Request) {
	s.debugTaskHandler.HandleGetDebugTask(w, r)
}

func (s *Server) handleListDebugTasksByAgent(w http.ResponseWriter, r *http.Request) {
	s.debugTaskHandler.HandleListDebugTasksByAgent(w, r)
}
