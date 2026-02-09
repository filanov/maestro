package engine_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("Scheduler", func() {
	var (
		scheduler *engine.Scheduler
		mockDB    *MockDB
		ctx       context.Context
		agentID   string
		clusterID string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockDB = NewMockDB()
		scheduler = engine.NewScheduler(mockDB)
		clusterID = "cluster-123"
		agentID = "agent-456"
	})

	Describe("GetTasksForAgent", func() {
		Context("when agent has no executions", func() {
			It("should return all tasks", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(3))
			})
		})

		Context("when agent has completed some tasks", func() {
			It("should return only pending tasks", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusSuccess},
				})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(2))
				Expect(tasks[0].ID).To(Equal("task-2"))
				Expect(tasks[1].ID).To(Equal("task-3"))
			})
		})

		Context("when a blocking task has failed", func() {
			It("should skip all subsequent tasks", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: true},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusFailed},
				})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(BeEmpty())
			})
		})

		Context("when a non-blocking task has failed", func() {
			It("should continue with subsequent tasks", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusFailed},
				})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(2))
				Expect(tasks[0].ID).To(Equal("task-2"))
				Expect(tasks[1].ID).To(Equal("task-3"))
			})
		})

		Context("when a task is currently running", func() {
			It("should skip the running task", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusRunning},
				})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0].ID).To(Equal("task-2"))
			})
		})

		Context("complex scenario with mixed states", func() {
			It("should handle blocking logic correctly", func() {
				mockDB.SetAgent(&models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				})

				mockDB.SetTasks([]*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: true},
					{ID: "task-4", ClusterID: clusterID, Order: 4, Blocking: false},
					{ID: "task-5", ClusterID: clusterID, Order: 5, Blocking: false},
				})

				mockDB.SetExecutions([]*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusSuccess},
					{TaskID: "task-2", AgentID: agentID, Status: models.ExecutionStatusFailed},
					{TaskID: "task-3", AgentID: agentID, Status: models.ExecutionStatusFailed},
				})

				tasks, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(BeEmpty())
			})
		})
	})
})

type MockDB struct {
	agent      *models.Agent
	tasks      []*models.Task
	executions []*models.TaskExecution
}

func NewMockDB() *MockDB {
	return &MockDB{
		tasks:      []*models.Task{},
		executions: []*models.TaskExecution{},
	}
}

func (m *MockDB) SetAgent(agent *models.Agent) {
	m.agent = agent
}

func (m *MockDB) SetTasks(tasks []*models.Task) {
	m.tasks = tasks
}

func (m *MockDB) SetExecutions(executions []*models.TaskExecution) {
	m.executions = executions
}

func (m *MockDB) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	if m.agent != nil && m.agent.ID == id {
		return m.agent, nil
	}
	return nil, db.ErrNotFound
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
	var result []*models.TaskExecution
	for _, exec := range m.executions {
		if exec.AgentID == agentID {
			result = append(result, exec)
		}
	}
	return result, nil
}

func (m *MockDB) Close() error                                                       { return nil }
func (m *MockDB) CreateCluster(ctx context.Context, cluster *models.Cluster) error   { return nil }
func (m *MockDB) GetCluster(ctx context.Context, id string) (*models.Cluster, error) { return nil, nil }
func (m *MockDB) ListClusters(ctx context.Context, limit, offset int) ([]*models.Cluster, int, error) {
	return nil, 0, nil
}
func (m *MockDB) DeleteCluster(ctx context.Context, id string) error         { return nil }
func (m *MockDB) CreateAgent(ctx context.Context, agent *models.Agent) error { return nil }
func (m *MockDB) UpdateAgent(ctx context.Context, id string, update *db.AgentUpdate) error {
	return nil
}
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
func (m *MockDB) GetTask(ctx context.Context, id string) (*models.Task, error)           { return nil, nil }
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
func (m *MockDB) UpsertExecution(ctx context.Context, execution *models.TaskExecution) error {
	return nil
}
func (m *MockDB) GetExecution(ctx context.Context, id string) (*models.TaskExecution, error) {
	return nil, nil
}
func (m *MockDB) ListExecutions(ctx context.Context, filters db.ExecutionFilters, limit, offset int) ([]*models.TaskExecution, int, error) {
	return nil, 0, nil
}
func (m *MockDB) DeleteAllExecutionsForAgent(ctx context.Context, agentID string) error { return nil }
func (m *MockDB) FailRunningTasksForAgent(ctx context.Context, agentID string) error    { return nil }
func (m *MockDB) CreateDebugTask(ctx context.Context, task *models.DebugTask) error     { return nil }
func (m *MockDB) GetDebugTask(ctx context.Context, id string) (*models.DebugTask, error) {
	return nil, nil
}
func (m *MockDB) ListDebugTasks(ctx context.Context, agentID string, limit, offset int) ([]*models.DebugTask, int, error) {
	return nil, 0, nil
}
func (m *MockDB) GetPendingDebugTasksForAgent(ctx context.Context, agentID string) ([]*models.DebugTask, error) {
	return nil, nil
}
func (m *MockDB) UpdateDebugTaskExecution(ctx context.Context, id string, status models.ExecutionStatus, output string, exitCode *int, error string) error {
	return nil
}
func (m *MockDB) DeleteCompletedDebugTasksOlderThan(ctx context.Context, threshold time.Time) error {
	return nil
}
func (m *MockDB) TimeoutPendingDebugTasks(ctx context.Context, threshold time.Time) error { return nil }
func (m *MockDB) DeleteExecutionsForDeletedTasksOlderThan(ctx context.Context, threshold time.Time) error {
	return nil
}
func (m *MockDB) DeleteOldExecutionsKeepLastN(ctx context.Context, keepN int) error { return nil }
