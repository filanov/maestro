package grpc_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/filanov/maestro/internal/api/grpc"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
	pb "github.com/filanov/maestro/proto/agent/v1"
)

var _ = Describe("gRPC Server", func() {
	var (
		server    *grpc.Server
		mockDB    *MockDB
		ctx       context.Context
		clusterID string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockDB = NewMockDB()
		server = grpc.NewServer(mockDB)
		clusterID = "cluster-123"

		mockDB.clusters[clusterID] = &models.Cluster{
			ID:   clusterID,
			Name: "Test Cluster",
		}
	})

	Describe("Register", func() {
		It("should register a new agent", func() {
			agentID := "550e8400-e29b-41d4-a716-446655440001"
			req := &pb.RegisterRequest{
				AgentId:   agentID,
				ClusterId: clusterID,
				Hostname:  "worker-01",
			}

			resp, err := server.Register(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetReset_()).To(BeFalse())

			Expect(mockDB.agents).To(HaveKey(agentID))
			Expect(mockDB.agents[agentID].Hostname).To(Equal("worker-01"))
		})

		It("should return reset=true for re-registration", func() {
			agentID := "550e8400-e29b-41d4-a716-446655440002"
			agent := &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
				Hostname:  "worker-01",
			}
			mockDB.agents[agent.ID] = agent
			mockDB.executions = append(mockDB.executions, &models.TaskExecution{
				AgentID: agent.ID,
				TaskID:  "task-1",
				Status:  models.ExecutionStatusSuccess,
			})

			req := &pb.RegisterRequest{
				AgentId:   agentID,
				ClusterId: clusterID,
				Hostname:  "worker-01",
			}

			resp, err := server.Register(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetReset_()).To(BeTrue())
			Expect(mockDB.executionDeleted).To(BeTrue())
		})

		It("should return NotFound for non-existent cluster", func() {
			req := &pb.RegisterRequest{
				AgentId:   "550e8400-e29b-41d4-a716-446655440003",
				ClusterId: "non-existent",
				Hostname:  "worker-01",
			}

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
			mockDB.agents[agentID] = &models.Agent{
				ID:            agentID,
				ClusterID:     clusterID,
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now().Add(-2 * time.Minute),
			}
		})

		It("should update heartbeat timestamp", func() {
			req := &pb.HeartbeatRequest{AgentId: agentID}

			resp, err := server.Heartbeat(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Acknowledged).To(BeTrue())
			Expect(mockDB.heartbeatUpdated).To(BeTrue())
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.HeartbeatRequest{AgentId: "non-existent"}

			_, err := server.Heartbeat(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})

	Describe("PollTasks", func() {
		var agentID string

		BeforeEach(func() {
			agentID = "agent-poll"
			mockDB.agents[agentID] = &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
			}

			mockDB.tasks = []*models.Task{
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
		})

		It("should return pending tasks", func() {
			req := &pb.PollTasksRequest{AgentId: agentID}

			resp, err := server.PollTasks(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Tasks).To(HaveLen(1))
			Expect(resp.Tasks[0].Name).To(Equal("Task 1"))
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.PollTasksRequest{AgentId: "non-existent"}

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

			mockDB.agents[agentID] = &models.Agent{
				ID:        agentID,
				ClusterID: clusterID,
			}

			mockDB.tasks = []*models.Task{
				{
					ID:        taskID,
					ClusterID: clusterID,
					Name:      "Report Task",
					Type:      models.TaskTypeExec,
					Config:    models.TaskConfig{Command: "echo test"},
				},
			}
		})

		It("should accept execution report", func() {
			req := &pb.ReportTaskExecutionRequest{
				AgentId:  agentID,
				TaskId:   taskID,
				Status:   pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
				Output:   "test output",
				ExitCode: 0,
			}

			resp, err := server.ReportTaskExecution(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Acknowledged).To(BeTrue())
			Expect(mockDB.executionReported).To(BeTrue())
		})

		It("should return NotFound for non-existent agent", func() {
			req := &pb.ReportTaskExecutionRequest{
				AgentId:  "non-existent",
				TaskId:   taskID,
				Status:   pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
				ExitCode: 0,
			}

			_, err := server.ReportTaskExecution(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})
})

type MockDB struct {
	clusters          map[string]*models.Cluster
	agents            map[string]*models.Agent
	tasks             []*models.Task
	executions        []*models.TaskExecution
	heartbeatUpdated  bool
	executionDeleted  bool
	executionReported bool
}

func NewMockDB() *MockDB {
	return &MockDB{
		clusters:   make(map[string]*models.Cluster),
		agents:     make(map[string]*models.Agent),
		tasks:      []*models.Task{},
		executions: []*models.TaskExecution{},
	}
}

func (m *MockDB) GetCluster(ctx context.Context, id string) (*models.Cluster, error) {
	if cluster, ok := m.clusters[id]; ok {
		return cluster, nil
	}
	return nil, db.ErrNotFound
}

func (m *MockDB) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	if agent, ok := m.agents[id]; ok {
		return agent, nil
	}
	return nil, db.ErrNotFound
}

func (m *MockDB) CreateAgent(ctx context.Context, agent *models.Agent) error {
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockDB) UpdateAgent(ctx context.Context, id string, update *db.AgentUpdate) error {
	if _, ok := m.agents[id]; !ok {
		return db.ErrNotFound
	}
	m.heartbeatUpdated = true
	return nil
}

func (m *MockDB) DeleteAllExecutionsForAgent(ctx context.Context, agentID string) error {
	m.executionDeleted = true
	return nil
}

func (m *MockDB) GetTasksForCluster(ctx context.Context, clusterID string) ([]*models.Task, error) {
	var result []*models.Task
	for _, task := range m.tasks {
		if task.ClusterID == clusterID {
			result = append(result, task)
		}
	}
	return result, nil
}

func (m *MockDB) GetExecutionsForAgent(ctx context.Context, agentID string) ([]*models.TaskExecution, error) {
	return []*models.TaskExecution{}, nil
}

func (m *MockDB) GetTask(ctx context.Context, id string) (*models.Task, error) {
	for _, task := range m.tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return nil, db.ErrNotFound
}

func (m *MockDB) UpsertExecution(ctx context.Context, execution *models.TaskExecution) error {
	m.executionReported = true
	return nil
}

func (m *MockDB) GetPendingDebugTasksForAgent(ctx context.Context, agentID string) ([]*models.DebugTask, error) {
	return []*models.DebugTask{}, nil
}

func (m *MockDB) GetDebugTask(ctx context.Context, id string) (*models.DebugTask, error) {
	return nil, db.ErrNotFound
}

func (m *MockDB) UpdateDebugTaskExecution(ctx context.Context, id string, status models.ExecutionStatus, output string, exitCode *int, error string) error {
	return nil
}

func (m *MockDB) Close() error                                                     { return nil }
func (m *MockDB) CreateCluster(ctx context.Context, cluster *models.Cluster) error { return nil }
func (m *MockDB) ListClusters(ctx context.Context, limit, offset int) ([]*models.Cluster, int, error) {
	return nil, 0, nil
}
func (m *MockDB) DeleteCluster(ctx context.Context, id string) error { return nil }
func (m *MockDB) ListAgents(ctx context.Context, clusterID string, status *models.AgentStatus, limit, offset int) ([]*models.Agent, int, error) {
	return nil, 0, nil
}
func (m *MockDB) DeleteAgent(ctx context.Context, id string) error { return nil }
func (m *MockDB) FindAgentsWithHeartbeatBefore(ctx context.Context, threshold time.Time) ([]*models.Agent, error) {
	return nil, nil
}
func (m *MockDB) UpdateAgentStatus(ctx context.Context, id string, status models.AgentStatus) error {
	return nil
}
func (m *MockDB) CreateTask(ctx context.Context, task *models.Task) error                { return nil }
func (m *MockDB) UpdateTask(ctx context.Context, id string, update *db.TaskUpdate) error { return nil }
func (m *MockDB) DeleteTask(ctx context.Context, id string) error                        { return nil }
func (m *MockDB) ListTasks(ctx context.Context, clusterID string, includeDeleted bool, limit, offset int) ([]*models.Task, int, error) {
	return nil, 0, nil
}
func (m *MockDB) ReorderTasks(ctx context.Context, clusterID string, taskIDs []string) error {
	return nil
}
func (m *MockDB) CreateExecution(ctx context.Context, execution *models.TaskExecution) error {
	return nil
}
func (m *MockDB) GetExecution(ctx context.Context, id string) (*models.TaskExecution, error) {
	return nil, nil
}
func (m *MockDB) ListExecutions(ctx context.Context, filters db.ExecutionFilters, limit, offset int) ([]*models.TaskExecution, int, error) {
	return nil, 0, nil
}
func (m *MockDB) FailRunningTasksForAgent(ctx context.Context, agentID string) error { return nil }
func (m *MockDB) CreateDebugTask(ctx context.Context, task *models.DebugTask) error  { return nil }
func (m *MockDB) ListDebugTasks(ctx context.Context, agentID string, limit, offset int) ([]*models.DebugTask, int, error) {
	return nil, 0, nil
}
func (m *MockDB) DeleteCompletedDebugTasksOlderThan(ctx context.Context, threshold time.Time) error {
	return nil
}
func (m *MockDB) TimeoutPendingDebugTasks(ctx context.Context, threshold time.Time) error { return nil }
func (m *MockDB) DeleteExecutionsForDeletedTasksOlderThan(ctx context.Context, threshold time.Time) error {
	return nil
}
func (m *MockDB) DeleteOldExecutionsKeepLastN(ctx context.Context, keepN int) error { return nil }
