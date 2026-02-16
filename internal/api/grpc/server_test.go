package grpc_test

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/filanov/maestro/api/proto/agent/v1"
	"github.com/filanov/maestro/internal/api/grpc"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("gRPC Server", func() {
	var (
		server    *grpc.Server
		mockDB    *db.MockDB
		mockCtrl  *gomock.Controller
		ctx       context.Context
		clusterID string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockDB = db.NewMockDB(mockCtrl)
		server = grpc.NewServer(mockDB)
		clusterID = "cluster-123"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Register", func() {
		It("should register a new agent", func() {
			agentID := "550e8400-e29b-41d4-a716-446655440001"
			req := &pb.RegisterRequest{
				AgentId:   agentID,
				ClusterId: clusterID,
				Hostname:  "worker-01",
			}

			cluster := &models.Cluster{
				ID:   clusterID,
				Name: "Test Cluster",
			}

			mockDB.EXPECT().GetCluster(ctx, clusterID).Return(cluster, nil)
			mockDB.EXPECT().GetAgent(ctx, agentID).Return(nil, db.ErrNotFound)
			mockDB.EXPECT().CreateAgent(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, agent *models.Agent) error {
				Expect(agent.ID).To(Equal(agentID))
				Expect(agent.ClusterID).To(Equal(clusterID))
				Expect(agent.Hostname).To(Equal("worker-01"))
				return nil
			})

			resp, err := server.Register(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetReset_()).To(BeFalse())
		})

		It("should return reset=true for re-registration", func() {
			agentID := "550e8400-e29b-41d4-a716-446655440002"
			agent := &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
				Hostname:  "worker-01",
			}
			cluster := &models.Cluster{
				ID:   clusterID,
				Name: "Test Cluster",
			}

			req := &pb.RegisterRequest{
				AgentId:   agentID,
				ClusterId: clusterID,
				Hostname:  "worker-01",
			}

			mockDB.EXPECT().GetCluster(ctx, clusterID).Return(cluster, nil)
			mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
			mockDB.EXPECT().DeleteAllExecutionsForAgent(ctx, agentID).Return(nil)
			mockDB.EXPECT().UpdateAgent(ctx, agentID, gomock.Any()).Return(nil)

			resp, err := server.Register(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetReset_()).To(BeTrue())
		})

		It("should return NotFound for non-existent cluster", func() {
			req := &pb.RegisterRequest{
				AgentId:   "550e8400-e29b-41d4-a716-446655440003",
				ClusterId: "non-existent",
				Hostname:  "worker-01",
			}

			mockDB.EXPECT().GetCluster(ctx, "non-existent").Return(nil, db.ErrNotFound)

			_, err := server.Register(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})

		It("should return InvalidArgument for invalid agent ID", func() {
			req := &pb.RegisterRequest{
				AgentId:   "not-a-uuid",
				ClusterId: clusterID,
				Hostname:  "worker-01",
			}

			_, err := server.Register(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})
	})

	Describe("Heartbeat", func() {
		var agentID string

		BeforeEach(func() {
			agentID = "agent-789"
		})

		It("should update heartbeat timestamp", func() {
			agent := &models.Agent{
				ID:            agentID,
				ClusterID:     clusterID,
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now().Add(-2 * time.Minute),
			}

			req := &pb.HeartbeatRequest{AgentId: agentID}

			mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
			mockDB.EXPECT().UpdateAgent(ctx, agentID, gomock.Any()).DoAndReturn(func(ctx context.Context, id string, update *db.AgentUpdate) error {
				Expect(update.LastHeartbeat).NotTo(BeNil())
				return nil
			})

			resp, err := server.Heartbeat(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Acknowledged).To(BeTrue())
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.HeartbeatRequest{AgentId: "non-existent"}

			mockDB.EXPECT().GetAgent(ctx, "non-existent").Return(nil, db.ErrNotFound)

			_, err := server.Heartbeat(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})

	Describe("PollTasks", func() {
		var agentID string

		BeforeEach(func() {
			agentID = "agent-poll"
		})

		It("should return pending tasks", func() {
			agent := &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
			}

			tasks := []*models.Task{
				{
					ID:        "task-1",
					ClusterID: clusterID,
					Name:      "Task 1",
					Type:      models.TaskTypeExec,
					Order:     1,
					Blocking:  false,
					Config: models.TaskConfig{
						Command: "echo hello",
						Timeout: 30 * time.Minute,
					},
				},
			}

			req := &pb.PollTasksRequest{AgentId: agentID}

			mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil).Times(2)
			mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
			mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return([]*models.TaskExecution{}, nil)

			resp, err := server.PollTasks(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Tasks).To(HaveLen(1))
			Expect(resp.Tasks[0].Name).To(Equal("Task 1"))
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.PollTasksRequest{AgentId: "non-existent"}

			mockDB.EXPECT().GetAgent(ctx, "non-existent").Return(nil, db.ErrNotFound)

			_, err := server.PollTasks(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})

	Describe("ReportTaskExecution", func() {
		var agentID, taskID string

		BeforeEach(func() {
			agentID = "agent-report"
			taskID = "task-report"
		})

		It("should accept execution report", func() {
			agent := &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
			}

			task := &models.Task{
				ID:        taskID,
				ClusterID: clusterID,
				Name:      "Report Task",
				Type:      models.TaskTypeExec,
				Config:    models.TaskConfig{Command: "echo test"},
			}

			req := &pb.ReportTaskExecutionRequest{
				AgentId:  agentID,
				TaskId:   taskID,
				Status:   pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
				Output:   "test output",
				ExitCode: 0,
			}

			mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
			mockDB.EXPECT().GetTask(ctx, taskID).Return(task, nil)
			mockDB.EXPECT().UpsertExecution(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, execution *models.TaskExecution) error {
				Expect(execution.AgentID).To(Equal(agentID))
				Expect(execution.TaskID).To(Equal(taskID))
				Expect(execution.Status).To(Equal(models.ExecutionStatusSuccess))
				Expect(execution.Output).To(Equal("test output"))
				return nil
			})

			resp, err := server.ReportTaskExecution(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Acknowledged).To(BeTrue())
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.ReportTaskExecutionRequest{
				AgentId:  "non-existent",
				TaskId:   taskID,
				Status:   pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
				ExitCode: 0,
			}

			mockDB.EXPECT().GetAgent(ctx, "non-existent").Return(nil, db.ErrNotFound)

			_, err := server.ReportTaskExecution(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})
})
