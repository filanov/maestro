package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/db/postgres"
	"github.com/filanov/maestro/internal/models"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var _ = Describe("Postgres DB", func() {
	var (
		database  *postgres.DB
		ctx       context.Context
		dbURL     string
		skipTests bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		dbURL = os.Getenv("MAESTRO_TEST_DB_URL")
		if dbURL == "" {
			skipTests = true
			Skip("MAESTRO_TEST_DB_URL not set, skipping integration tests")
			return
		}

		var err error
		database, err = postgres.New(dbURL, 10, 2, 5*time.Minute, 5*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		cleanDatabase(dbURL)
		runMigrations(dbURL)
	})

	AfterEach(func() {
		if !skipTests && database != nil {
			database.Close()
		}
	})

	Describe("Cluster Operations", func() {
		It("should create and retrieve a cluster", func() {
			cluster := &models.Cluster{
				Name:        "Test Cluster",
				Description: "A test cluster",
			}

			err := database.CreateCluster(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.ID).NotTo(BeEmpty())

			retrieved, err := database.GetCluster(ctx, cluster.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Test Cluster"))
			Expect(retrieved.Description).To(Equal("A test cluster"))
		})

		It("should return ErrNotFound for non-existent cluster", func() {
			_, err := database.GetCluster(ctx, "non-existent-id")
			Expect(err).To(Equal(db.ErrNotFound))
		})

		It("should list clusters with pagination", func() {
			for i := 0; i < 5; i++ {
				cluster := &models.Cluster{Name: "Cluster " + string(rune('A'+i))}
				Expect(database.CreateCluster(ctx, cluster)).To(Succeed())
			}

			clusters, total, err := database.ListClusters(ctx, 3, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusters).To(HaveLen(3))
			Expect(total).To(Equal(5))

			clusters, total, err = database.ListClusters(ctx, 3, 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusters).To(HaveLen(2))
			Expect(total).To(Equal(5))
		})

		It("should delete a cluster", func() {
			cluster := &models.Cluster{Name: "To Delete"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())

			err := database.DeleteCluster(ctx, cluster.ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.GetCluster(ctx, cluster.ID)
			Expect(err).To(Equal(db.ErrNotFound))
		})
	})

	Describe("Agent Operations", func() {
		var clusterID string

		BeforeEach(func() {
			cluster := &models.Cluster{Name: "Agent Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())
			clusterID = cluster.ID
		})

		It("should create and retrieve an agent", func() {
			agent := &models.Agent{
				ID:            "agent-123",
				ClusterID:     clusterID,
				Hostname:      "worker-01",
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now(),
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}

			err := database.CreateAgent(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := database.GetAgent(ctx, agent.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Hostname).To(Equal("worker-01"))
			Expect(retrieved.Status).To(Equal(models.AgentStatusActive))
		})

		It("should update agent heartbeat", func() {
			agent := &models.Agent{
				ID:            "agent-456",
				ClusterID:     clusterID,
				Hostname:      "worker-02",
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now().Add(-5 * time.Minute),
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent)).To(Succeed())

			newHeartbeat := time.Now()
			err := database.UpdateAgent(ctx, agent.ID, &db.AgentUpdate{
				LastHeartbeat: &newHeartbeat,
			})
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := database.GetAgent(ctx, agent.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.LastHeartbeat).To(BeTemporally("~", newHeartbeat, time.Second))
		})

		It("should find agents with old heartbeats", func() {
			oldTime := time.Now().Add(-10 * time.Minute)
			recentTime := time.Now().Add(-1 * time.Minute)

			agent1 := &models.Agent{
				ID:            "agent-old",
				ClusterID:     clusterID,
				Hostname:      "worker-old",
				Status:        models.AgentStatusActive,
				LastHeartbeat: oldTime,
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent1)).To(Succeed())

			agent2 := &models.Agent{
				ID:            "agent-recent",
				ClusterID:     clusterID,
				Hostname:      "worker-recent",
				Status:        models.AgentStatusActive,
				LastHeartbeat: recentTime,
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent2)).To(Succeed())

			threshold := time.Now().Add(-5 * time.Minute)
			agents, err := database.FindAgentsWithHeartbeatBefore(ctx, threshold)
			Expect(err).NotTo(HaveOccurred())
			Expect(agents).To(HaveLen(1))
			Expect(agents[0].ID).To(Equal("agent-old"))
		})
	})

	Describe("Task Operations", func() {
		var clusterID string

		BeforeEach(func() {
			cluster := &models.Cluster{Name: "Task Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())
			clusterID = cluster.ID
		})

		It("should create tasks with auto-incrementing order", func() {
			task1 := &models.Task{
				ClusterID: clusterID,
				Name:      "Task 1",
				Type:      models.TaskTypeExec,
				Blocking:  false,
				Config: models.TaskConfig{
					Command: "echo hello",
					Timeout: 30 * time.Minute,
				},
			}
			Expect(database.CreateTask(ctx, task1)).To(Succeed())
			Expect(task1.Order).To(Equal(1))

			task2 := &models.Task{
				ClusterID: clusterID,
				Name:      "Task 2",
				Type:      models.TaskTypeExec,
				Blocking:  true,
				Config: models.TaskConfig{
					Command: "echo world",
					Timeout: 30 * time.Minute,
				},
			}
			Expect(database.CreateTask(ctx, task2)).To(Succeed())
			Expect(task2.Order).To(Equal(2))
		})

		It("should soft delete and reorder tasks", func() {
			task1 := &models.Task{ClusterID: clusterID, Name: "Task 1", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 1"}}
			task2 := &models.Task{ClusterID: clusterID, Name: "Task 2", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 2"}}
			task3 := &models.Task{ClusterID: clusterID, Name: "Task 3", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 3"}}

			Expect(database.CreateTask(ctx, task1)).To(Succeed())
			Expect(database.CreateTask(ctx, task2)).To(Succeed())
			Expect(database.CreateTask(ctx, task3)).To(Succeed())

			err := database.DeleteTask(ctx, task2.ID)
			Expect(err).NotTo(HaveOccurred())

			tasks, err := database.GetTasksForCluster(ctx, clusterID)
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks).To(HaveLen(2))
			Expect(tasks[0].Name).To(Equal("Task 1"))
			Expect(tasks[0].Order).To(Equal(1))
			Expect(tasks[1].Name).To(Equal("Task 3"))
			Expect(tasks[1].Order).To(Equal(2))
		})

		It("should reorder tasks", func() {
			task1 := &models.Task{ClusterID: clusterID, Name: "Task 1", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 1"}}
			task2 := &models.Task{ClusterID: clusterID, Name: "Task 2", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 2"}}
			task3 := &models.Task{ClusterID: clusterID, Name: "Task 3", Type: models.TaskTypeExec, Config: models.TaskConfig{Command: "echo 3"}}

			Expect(database.CreateTask(ctx, task1)).To(Succeed())
			Expect(database.CreateTask(ctx, task2)).To(Succeed())
			Expect(database.CreateTask(ctx, task3)).To(Succeed())

			err := database.ReorderTasks(ctx, clusterID, []string{task3.ID, task1.ID, task2.ID})
			Expect(err).NotTo(HaveOccurred())

			tasks, err := database.GetTasksForCluster(ctx, clusterID)
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks[0].Name).To(Equal("Task 3"))
			Expect(tasks[1].Name).To(Equal("Task 1"))
			Expect(tasks[2].Name).To(Equal("Task 2"))
		})
	})

	Describe("Execution Operations", func() {
		var clusterID, agentID, taskID string

		BeforeEach(func() {
			cluster := &models.Cluster{Name: "Execution Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())
			clusterID = cluster.ID

			agent := &models.Agent{
				ID:            "exec-agent",
				ClusterID:     clusterID,
				Hostname:      "exec-worker",
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now(),
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent)).To(Succeed())
			agentID = agent.ID

			task := &models.Task{
				ClusterID: clusterID,
				Name:      "Exec Task",
				Type:      models.TaskTypeExec,
				Config:    models.TaskConfig{Command: "echo test"},
			}
			Expect(database.CreateTask(ctx, task)).To(Succeed())
			taskID = task.ID
		})

		It("should upsert execution results", func() {
			execution := &models.TaskExecution{
				TaskID:    taskID,
				AgentID:   agentID,
				Status:    models.ExecutionStatusRunning,
				StartedAt: time.Now(),
			}

			err := database.UpsertExecution(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			execution.Status = models.ExecutionStatusSuccess
			exitCode := 0
			execution.ExitCode = &exitCode
			completed := time.Now()
			execution.CompletedAt = &completed

			err = database.UpsertExecution(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			executions, err := database.GetExecutionsForAgent(ctx, agentID)
			Expect(err).NotTo(HaveOccurred())
			Expect(executions).To(HaveLen(1))
			Expect(executions[0].Status).To(Equal(models.ExecutionStatusSuccess))
		})

		It("should prevent duplicate executions via unique constraint", func() {
			execution1 := &models.TaskExecution{
				TaskID:    taskID,
				AgentID:   agentID,
				Status:    models.ExecutionStatusSuccess,
				StartedAt: time.Now(),
			}
			err := database.CreateExecution(ctx, execution1)
			Expect(err).NotTo(HaveOccurred())

			execution2 := &models.TaskExecution{
				TaskID:    taskID,
				AgentID:   agentID,
				Status:    models.ExecutionStatusSuccess,
				StartedAt: time.Now(),
			}
			err = database.CreateExecution(ctx, execution2)
			Expect(err).To(HaveOccurred())
		})
	})
})

func cleanDatabase(dbURL string) {
	conn, err := sql.Open("postgres", dbURL)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()

	_, err = conn.Exec(`
		DROP TABLE IF EXISTS debug_tasks CASCADE;
		DROP TABLE IF EXISTS task_executions CASCADE;
		DROP TABLE IF EXISTS tasks CASCADE;
		DROP TABLE IF EXISTS agents CASCADE;
		DROP TABLE IF EXISTS clusters CASCADE;
		DROP TABLE IF EXISTS schema_migrations CASCADE;
	`)
	Expect(err).NotTo(HaveOccurred())
}

func runMigrations(dbURL string) {
	m, err := migrate.New("file://../../../migrations", dbURL)
	Expect(err).NotTo(HaveOccurred())
	defer m.Close()

	err = m.Up()
	Expect(err).NotTo(HaveOccurred())
}
