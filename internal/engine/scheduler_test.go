package engine_test

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("Scheduler", func() {
	var (
		scheduler *engine.Scheduler
		mockDB    *db.MockDB
		mockCtrl  *gomock.Controller
		ctx       context.Context
		agentID   string
		clusterID string
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

		Context("with scheduled tasks", func() {
			It("should include scheduled task on first run (no prior execution)", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{
						ID:               "task-1",
						ClusterID:        clusterID,
						Order:            1,
						ScheduleEnabled:  true,
						ScheduleInterval: 5 * time.Minute,
					},
				}
				executions := []*models.TaskExecution{} // No prior execution

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ID).To(Equal("task-1"))
			})

			It("should include scheduled task when interval has elapsed", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{
						ID:               "task-1",
						ClusterID:        clusterID,
						Order:            1,
						ScheduleEnabled:  true,
						ScheduleInterval: 5 * time.Minute,
					},
				}
				executions := []*models.TaskExecution{
					{
						TaskID:    "task-1",
						AgentID:   agentID,
						Status:    models.ExecutionStatusSuccess,
						StartedAt: time.Now().Add(-10 * time.Minute), // 10 minutes ago (> 5 min interval)
					},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].ID).To(Equal("task-1"))
			})

			It("should skip scheduled task when interval has not elapsed", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{
						ID:               "task-1",
						ClusterID:        clusterID,
						Order:            1,
						ScheduleEnabled:  true,
						ScheduleInterval: 5 * time.Minute,
					},
				}
				executions := []*models.TaskExecution{
					{
						TaskID:    "task-1",
						AgentID:   agentID,
						Status:    models.ExecutionStatusSuccess,
						StartedAt: time.Now().Add(-2 * time.Minute), // 2 minutes ago (< 5 min interval)
					},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})

			It("should skip scheduled task when currently running", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{
						ID:               "task-1",
						ClusterID:        clusterID,
						Order:            1,
						ScheduleEnabled:  true,
						ScheduleInterval: 5 * time.Minute,
					},
				}
				executions := []*models.TaskExecution{
					{
						TaskID:    "task-1",
						AgentID:   agentID,
						Status:    models.ExecutionStatusRunning,
						StartedAt: time.Now().Add(-10 * time.Minute), // Started long ago but still running
					},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})

			It("should handle mix of scheduled and regular tasks", func() {
				agent := &models.Agent{
					ID:        agentID,
					ClusterID: clusterID,
				}
				tasks := []*models.Task{
					{
						ID:              "task-1",
						ClusterID:       clusterID,
						Order:           1,
						ScheduleEnabled: false, // Regular task
					},
					{
						ID:               "task-2",
						ClusterID:        clusterID,
						Order:            2,
						ScheduleEnabled:  true, // Scheduled task
						ScheduleInterval: 5 * time.Minute,
					},
					{
						ID:              "task-3",
						ClusterID:       clusterID,
						Order:           3,
						ScheduleEnabled: false, // Regular task
					},
				}
				executions := []*models.TaskExecution{
					{
						TaskID:  "task-1",
						AgentID: agentID,
						Status:  models.ExecutionStatusSuccess, // Regular task completed
					},
					{
						TaskID:    "task-2",
						AgentID:   agentID,
						Status:    models.ExecutionStatusSuccess,
						StartedAt: time.Now().Add(-10 * time.Minute), // Scheduled task due for re-run
					},
				}

				mockDB.EXPECT().GetAgent(ctx, agentID).Return(agent, nil)
				mockDB.EXPECT().GetTasksForCluster(ctx, clusterID).Return(tasks, nil)
				mockDB.EXPECT().GetExecutionsForAgent(ctx, agentID).Return(executions, nil)

				result, err := scheduler.GetTasksForAgent(ctx, agentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].ID).To(Equal("task-2")) // Scheduled task due for re-run
				Expect(result[1].ID).To(Equal("task-3")) // Regular task pending
			})
		})
	})
})
