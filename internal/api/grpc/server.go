package grpc

import (
	"context"
	"time"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
	pb "github.com/filanov/maestro/proto/agent/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedAgentServiceServer
	db        db.DB
	scheduler *engine.Scheduler
}

func NewServer(database db.DB) *Server {
	return &Server{
		db:        database,
		scheduler: engine.NewScheduler(database),
	}
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if _, err := uuid.Parse(req.AgentId); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent_id format")
	}

	if req.ClusterId == "" || req.Hostname == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_id and hostname are required")
	}

	if _, err := s.db.GetCluster(ctx, req.ClusterId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "cluster not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	_, err := s.db.GetAgent(ctx, req.AgentId)

	if err == db.ErrNotFound {
		now := time.Now()
		agent := &models.Agent{
			ID:            req.AgentId,
			ClusterID:     req.ClusterId,
			Hostname:      req.Hostname,
			Status:        models.AgentStatusActive,
			LastHeartbeat: now,
			RegisteredAt:  now,
			LastResetAt:   now,
		}
		if err := s.db.CreateAgent(ctx, agent); err != nil {
			return nil, status.Error(codes.Internal, "failed to create agent")
		}
		return &pb.RegisterResponse{Reset_: false}, nil
	}

	if err != nil {
		return nil, status.Error(codes.Internal, "database error")
	}

	if err := s.db.DeleteAllExecutionsForAgent(ctx, req.AgentId); err != nil {
		return nil, status.Error(codes.Internal, "failed to reset agent")
	}

	now := time.Now()
	if err := s.db.UpdateAgent(ctx, req.AgentId, &db.AgentUpdate{
		Status:        &[]models.AgentStatus{models.AgentStatusActive}[0],
		LastHeartbeat: &now,
		LastResetAt:   &now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to update agent")
	}

	return &pb.RegisterResponse{Reset_: true}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if _, err := s.db.GetAgent(ctx, req.AgentId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	now := time.Now()
	activeStatus := models.AgentStatusActive
	if err := s.db.UpdateAgent(ctx, req.AgentId, &db.AgentUpdate{
		LastHeartbeat: &now,
		Status:        &activeStatus,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to update heartbeat")
	}

	return &pb.HeartbeatResponse{Acknowledged: true}, nil
}

func (s *Server) PollTasks(ctx context.Context, req *pb.PollTasksRequest) (*pb.PollTasksResponse, error) {
	if _, err := s.db.GetAgent(ctx, req.AgentId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	tasks, err := s.scheduler.GetTasksForAgent(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get tasks")
	}

	pbTasks := make([]*pb.Task, 0, len(tasks))
	for _, task := range tasks {
		pbTask := &pb.Task{
			Id:       task.ID,
			Name:     task.Name,
			Type:     pb.TaskType_TASK_TYPE_EXEC,
			Blocking: task.Blocking,
		}

		if task.Type == models.TaskTypeExec {
			timeoutSeconds := int32(task.Config.Timeout.Seconds())
			if timeoutSeconds == 0 {
				timeoutSeconds = 1800
			}
			pbTask.Config = &pb.Task_Exec{
				Exec: &pb.ExecConfig{
					Command:        task.Config.Command,
					TimeoutSeconds: timeoutSeconds,
					WorkingDir:     task.Config.WorkingDir,
				},
			}
		}

		pbTasks = append(pbTasks, pbTask)
	}

	return &pb.PollTasksResponse{Tasks: pbTasks}, nil
}

func (s *Server) ReportTaskExecution(ctx context.Context, req *pb.ReportTaskExecutionRequest) (*pb.ReportTaskExecutionResponse, error) {
	if _, err := s.db.GetAgent(ctx, req.AgentId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	if _, err := s.db.GetTask(ctx, req.TaskId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "task not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	execStatus := mapProtoStatusToModel(req.Status)
	if execStatus == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid execution status")
	}

	execution := &models.TaskExecution{
		TaskID:    req.TaskId,
		AgentID:   req.AgentId,
		Status:    execStatus,
		Output:    req.Output,
		StartedAt: time.Now(),
	}

	if req.ExitCode != 0 || req.Status != pb.ExecutionStatus_EXECUTION_STATUS_RUNNING {
		exitCode := int(req.ExitCode)
		execution.ExitCode = &exitCode
	}

	if req.Status != pb.ExecutionStatus_EXECUTION_STATUS_RUNNING {
		now := time.Now()
		execution.CompletedAt = &now
	}

	if req.Error != "" {
		execution.Error = req.Error
	}

	if err := s.db.UpsertExecution(ctx, execution); err != nil {
		return nil, status.Error(codes.Internal, "failed to report execution")
	}

	return &pb.ReportTaskExecutionResponse{Acknowledged: true}, nil
}

func (s *Server) PollDebugTasks(ctx context.Context, req *pb.PollDebugTasksRequest) (*pb.PollDebugTasksResponse, error) {
	if _, err := s.db.GetAgent(ctx, req.AgentId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	tasks, err := s.db.GetPendingDebugTasksForAgent(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get debug tasks")
	}

	pbTasks := make([]*pb.DebugTask, 0, len(tasks))
	for _, task := range tasks {
		pbTasks = append(pbTasks, &pb.DebugTask{
			Id:             task.ID,
			Command:        task.Command,
			TimeoutSeconds: 300,
		})
	}

	return &pb.PollDebugTasksResponse{DebugTasks: pbTasks}, nil
}

func (s *Server) ReportDebugTaskExecution(ctx context.Context, req *pb.ReportDebugTaskExecutionRequest) (*pb.ReportDebugTaskExecutionResponse, error) {
	if _, err := s.db.GetAgent(ctx, req.AgentId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "agent not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	if _, err := s.db.GetDebugTask(ctx, req.DebugTaskId); err != nil {
		if err == db.ErrNotFound {
			return nil, status.Error(codes.NotFound, "debug task not found")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	execStatus := mapProtoStatusToModel(req.Status)
	if execStatus == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid execution status")
	}

	var exitCode *int
	if req.ExitCode != 0 || req.Status != pb.ExecutionStatus_EXECUTION_STATUS_RUNNING {
		ec := int(req.ExitCode)
		exitCode = &ec
	}

	if err := s.db.UpdateDebugTaskExecution(ctx, req.DebugTaskId, execStatus, req.Output, exitCode, req.Error); err != nil {
		return nil, status.Error(codes.Internal, "failed to update debug task")
	}

	return &pb.ReportDebugTaskExecutionResponse{Acknowledged: true}, nil
}

func mapProtoStatusToModel(status pb.ExecutionStatus) models.ExecutionStatus {
	switch status {
	case pb.ExecutionStatus_EXECUTION_STATUS_RUNNING:
		return models.ExecutionStatusRunning
	case pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS:
		return models.ExecutionStatusSuccess
	case pb.ExecutionStatus_EXECUTION_STATUS_FAILED:
		return models.ExecutionStatusFailed
	default:
		return ""
	}
}

func StartGRPCServer(addr string, database db.DB) (*grpc.Server, error) {
	server := grpc.NewServer()
	pb.RegisterAgentServiceServer(server, NewServer(database))
	return server, nil
}
