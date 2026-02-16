package engine_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("Scheduler", func() {
	var (
		scheduler  *engine.Scheduler
		mockDB     *db.MockDB
		mockCtrl   *gomock.Controller
		ctx        context.Context
		agentID    string
		clusterID  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockDB = db.NewMockDB(mockCtrl)
		scheduler = engine.NewScheduler(mockDB)
		clusterID = "cluster-123"
		agentID = "agent-456"
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("GetTasksForAgent", func() {
		Context("when agent has no executions", func() {
			It("should return all tasks", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				}
				executions := []*models.TaskExecution{}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(3))
			})
		})

		Context("when agent has completed some tasks", func() {
			It("should return only pending tasks", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				}
				executions := []*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusSuccess},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].ID).To(Equal("task-2"))
				Expect(result[1].ID).To(Equal("task-3"))
			})
		})

		Context("when a blocking task has failed", func() {
			It("should skip all subsequent tasks", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: true},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				}
				executions := []*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusFailed},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})
		})

		Context("when a non-blocking task has failed", func() {
			It("should continue with subsequent tasks", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: false},
				}
				executions := []*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusFailed},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].ID).To(Equal("task-2"))
				Expect(result[1].ID).To(Equal("task-3"))
			})
		})

		Context("when a task is currently running", func() {
			It("should skip the running task", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
				}
				executions := []*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusRunning},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ID).To(Equal("task-2"))
			})
		})

		Context("complex scenario with mixed states", func() {
			It("should handle blocking logic correctly", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{ID: "task-1", ClusterID: clusterID, Order: 1, Blocking: false},
					{ID: "task-2", ClusterID: clusterID, Order: 2, Blocking: false},
					{ID: "task-3", ClusterID: clusterID, Order: 3, Blocking: true},
					{ID: "task-4", ClusterID: clusterID, Order: 4, Blocking: false},
					{ID: "task-5", ClusterID: clusterID, Order: 5, Blocking: false},
				}
				executions := []*models.TaskExecution{
					{TaskID: "task-1", AgentID: agentID, Status: models.ExecutionStatusSuccess},
					{TaskID: "task-2", AgentID: agentID, Status: models.ExecutionStatusFailed},
					{TaskID: "task-3", AgentID: agentID, Status: models.ExecutionStatusFailed},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})
		})
	})
})
