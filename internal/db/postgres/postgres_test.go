package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/google/uuid"
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
			_, err := database.GetCluster(ctx, uuid.New().String())
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
				ID:            uuid.New().String(),
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
				ID:            uuid.New().String(),
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

			agent1ID := uuid.New().String()
			agent1 := &models.Agent{
				ID:            agent1ID,
				ClusterID:     clusterID,
				Hostname:      "worker-old",
				Status:        models.AgentStatusActive,
				LastHeartbeat: oldTime,
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent1)).To(Succeed())

			agent2 := &models.Agent{
				ID:            uuid.New().String(),
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
			Expect(agents[0].ID).To(Equal(agent1ID))
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

			agentID = uuid.New().String()
			agent := &models.Agent{
				ID:            agentID,
				ClusterID:     clusterID,
				Hostname:      "exec-worker",
				Status:        models.AgentStatusActive,
				LastHeartbeat: time.Now(),
				RegisteredAt:  time.Now(),
				LastResetAt:   time.Now(),
			}
			Expect(database.CreateAgent(ctx, agent)).To(Succeed())

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
				ClusterID: clusterID,
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
				ClusterID: clusterID,
				Status:    models.ExecutionStatusSuccess,
				StartedAt: time.Now(),
			}
			err := database.CreateExecution(ctx, execution1)
			Expect(err).NotTo(HaveOccurred())

			execution2 := &models.TaskExecution{
				TaskID:    taskID,
				AgentID:   agentID,
				ClusterID: clusterID,
				Status:    models.ExecutionStatusSuccess,
				StartedAt: time.Now(),
			}
			err = database.CreateExecution(ctx, execution2)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Template Operations", func() {
		It("should create and retrieve a template", func() {
			template := &models.Template{
				Name:        "Test Template",
				Description: "A test template",
			}

			err := database.CreateTemplate(ctx, template)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.ID).NotTo(BeEmpty())

			retrieved, err := database.GetTemplate(ctx, template.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Test Template"))
			Expect(retrieved.Description).To(Equal("A test template"))
		})

		It("should update a template", func() {
			template := &models.Template{
				Name:        "Original Name",
				Description: "Original Description",
			}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			newName := "Updated Name"
			update := &db.TemplateUpdate{Name: &newName}
			err := database.UpdateTemplate(ctx, template.ID, update)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := database.GetTemplate(ctx, template.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Updated Name"))
			Expect(retrieved.Description).To(Equal("Original Description"))
		})

		It("should list templates with pagination", func() {
			for i := 0; i < 5; i++ {
				template := &models.Template{Name: "Template " + string(rune('A'+i))}
				Expect(database.CreateTemplate(ctx, template)).To(Succeed())
			}

			templates, total, err := database.ListTemplates(ctx, 3, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(templates).To(HaveLen(3))
			Expect(total).To(Equal(5))

			templates, total, err = database.ListTemplates(ctx, 3, 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(templates).To(HaveLen(2))
			Expect(total).To(Equal(5))
		})

		It("should delete a template", func() {
			template := &models.Template{Name: "To Delete"}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			err := database.DeleteTemplate(ctx, template.ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.GetTemplate(ctx, template.ID)
			Expect(err).To(Equal(db.ErrNotFound))
		})

		It("should create and list template tasks", func() {
			template := &models.Template{Name: "Test Template"}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			task1 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Task 1",
				Type:       models.TaskTypeExec,
				Order:      1,
				Blocking:   true,
				Config:     models.TaskConfig{Command: "echo hello"},
			}
			Expect(database.CreateTemplateTask(ctx, task1)).To(Succeed())

			task2 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Task 2",
				Type:       models.TaskTypeExec,
				Order:      2,
				Blocking:   false,
				Config:     models.TaskConfig{Command: "echo world"},
			}
			Expect(database.CreateTemplateTask(ctx, task2)).To(Succeed())

			tasks, total, err := database.ListTemplateTasks(ctx, template.ID, 10, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks).To(HaveLen(2))
			Expect(total).To(Equal(2))
			Expect(tasks[0].Name).To(Equal("Task 1"))
			Expect(tasks[1].Name).To(Equal("Task 2"))
		})

		It("should import template to cluster", func() {
			cluster := &models.Cluster{Name: "Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())

			template := &models.Template{Name: "Import Template"}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			task1 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Task 1",
				Type:       models.TaskTypeExec,
				Order:      1,
				Blocking:   true,
				Config:     models.TaskConfig{Command: "cmd1"},
			}
			Expect(database.CreateTemplateTask(ctx, task1)).To(Succeed())

			task2 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Task 2",
				Type:       models.TaskTypeExec,
				Order:      2,
				Blocking:   false,
				Config:     models.TaskConfig{Command: "cmd2"},
			}
			Expect(database.CreateTemplateTask(ctx, task2)).To(Succeed())

			err := database.ImportTemplateToCluster(ctx, cluster.ID, template.ID)
			Expect(err).NotTo(HaveOccurred())

			clusterTasks, err := database.GetTasksForCluster(ctx, cluster.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterTasks).To(HaveLen(2))
			Expect(clusterTasks[0].Name).To(Equal("Task 1"))
			Expect(clusterTasks[0].Order).To(Equal(1))
			Expect(clusterTasks[1].Name).To(Equal("Task 2"))
			Expect(clusterTasks[1].Order).To(Equal(2))
		})

		It("should import template to cluster with existing tasks", func() {
			cluster := &models.Cluster{Name: "Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())

			existingTask := &models.Task{
				ClusterID: cluster.ID,
				Name:      "Existing Task",
				Type:      models.TaskTypeExec,
				Order:     1,
				Config:    models.TaskConfig{Command: "existing"},
			}
			Expect(database.CreateTask(ctx, existingTask)).To(Succeed())

			template := &models.Template{Name: "Import Template"}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			templateTask := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Template Task",
				Type:       models.TaskTypeExec,
				Order:      1,
				Blocking:   false,
				Config:     models.TaskConfig{Command: "template"},
			}
			Expect(database.CreateTemplateTask(ctx, templateTask)).To(Succeed())

			err := database.ImportTemplateToCluster(ctx, cluster.ID, template.ID)
			Expect(err).NotTo(HaveOccurred())

			clusterTasks, err := database.GetTasksForCluster(ctx, cluster.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterTasks).To(HaveLen(2))
			Expect(clusterTasks[0].Name).To(Equal("Existing Task"))
			Expect(clusterTasks[0].Order).To(Equal(1))
			Expect(clusterTasks[1].Name).To(Equal("Template Task"))
			Expect(clusterTasks[1].Order).To(Equal(2))
		})

		It("should export cluster to template", func() {
			cluster := &models.Cluster{Name: "Test Cluster"}
			Expect(database.CreateCluster(ctx, cluster)).To(Succeed())

			task1 := &models.Task{
				ClusterID: cluster.ID,
				Name:      "Task 1",
				Type:      models.TaskTypeExec,
				Order:     1,
				Blocking:  true,
				Config:    models.TaskConfig{Command: "cmd1"},
			}
			Expect(database.CreateTask(ctx, task1)).To(Succeed())

			task2 := &models.Task{
				ClusterID: cluster.ID,
				Name:      "Task 2",
				Type:      models.TaskTypeExec,
				Order:     2,
				Blocking:  false,
				Config:    models.TaskConfig{Command: "cmd2"},
			}
			Expect(database.CreateTask(ctx, task2)).To(Succeed())

			template := &models.Template{
				Name:        "Exported Template",
				Description: "Exported from cluster",
			}
			err := database.ExportClusterToTemplate(ctx, cluster.ID, template)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.ID).NotTo(BeEmpty())

			templateTasks, err := database.GetTemplateTasksForTemplate(ctx, template.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(templateTasks).To(HaveLen(2))
			Expect(templateTasks[0].Name).To(Equal("Task 1"))
			Expect(templateTasks[0].Order).To(Equal(1))
			Expect(templateTasks[1].Name).To(Equal("Task 2"))
			Expect(templateTasks[1].Order).To(Equal(2))
		})

		It("should handle import/export round trip", func() {
			cluster1 := &models.Cluster{Name: "Source Cluster"}
			Expect(database.CreateCluster(ctx, cluster1)).To(Succeed())

			task := &models.Task{
				ClusterID: cluster1.ID,
				Name:      "Original Task",
				Type:      models.TaskTypeExec,
				Order:     1,
				Blocking:  true,
				Config:    models.TaskConfig{Command: "test cmd"},
			}
			Expect(database.CreateTask(ctx, task)).To(Succeed())

			template := &models.Template{Name: "Round Trip"}
			err := database.ExportClusterToTemplate(ctx, cluster1.ID, template)
			Expect(err).NotTo(HaveOccurred())

			cluster2 := &models.Cluster{Name: "Target Cluster"}
			Expect(database.CreateCluster(ctx, cluster2)).To(Succeed())

			err = database.ImportTemplateToCluster(ctx, cluster2.ID, template.ID)
			Expect(err).NotTo(HaveOccurred())

			cluster2Tasks, err := database.GetTasksForCluster(ctx, cluster2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster2Tasks).To(HaveLen(1))
			Expect(cluster2Tasks[0].Name).To(Equal("Original Task"))
			Expect(cluster2Tasks[0].Blocking).To(BeTrue())
			Expect(cluster2Tasks[0].Config.Command).To(Equal("test cmd"))
		})

		It("should cascade delete template tasks when template is deleted", func() {
			template := &models.Template{Name: "Cascade Test"}
			Expect(database.CreateTemplate(ctx, template)).To(Succeed())

			task := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Task to be deleted",
				Type:       models.TaskTypeExec,
				Order:      1,
				Config:     models.TaskConfig{Command: "test"},
			}
			Expect(database.CreateTemplateTask(ctx, task)).To(Succeed())

			err := database.DeleteTemplate(ctx, template.ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = database.GetTemplateTask(ctx, task.ID)
			Expect(err).To(Equal(db.ErrNotFound))
		})

		It("should complete full workflow: create template, add tasks, create cluster, import, verify", func() {
			template := &models.Template{
				Name:        "Deployment Template",
				Description: "Standard deployment tasks",
			}
			err := database.CreateTemplate(ctx, template)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.ID).NotTo(BeEmpty())

			task1 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Install dependencies",
				Type:       models.TaskTypeExec,
				Order:      1,
				Blocking:   true,
				Config: models.TaskConfig{
					Command:    "apt-get update && apt-get install -y nginx",
					Timeout:    300 * time.Second,
					WorkingDir: "/tmp",
				},
			}
			err = database.CreateTemplateTask(ctx, task1)
			Expect(err).NotTo(HaveOccurred())
			Expect(task1.ID).NotTo(BeEmpty())

			task2 := &models.TemplateTask{
				TemplateID: template.ID,
				Name:       "Start service",
				Type:       models.TaskTypeExec,
				Order:      2,
				Blocking:   false,
				Config: models.TaskConfig{
					Command: "systemctl start nginx",
					Timeout: 60 * time.Second,
				},
			}
			err = database.CreateTemplateTask(ctx, task2)
			Expect(err).NotTo(HaveOccurred())
			Expect(task2.ID).NotTo(BeEmpty())

			cluster := &models.Cluster{
				Name:        "Production Cluster",
				Description: "Main production environment",
			}
			err = database.CreateCluster(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.ID).NotTo(BeEmpty())

			err = database.ImportTemplateToCluster(ctx, cluster.ID, template.ID)
			Expect(err).NotTo(HaveOccurred())

			clusterTasks, err := database.GetTasksForCluster(ctx, cluster.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusterTasks).To(HaveLen(2))

			Expect(clusterTasks[0].Name).To(Equal("Install dependencies"))
			Expect(clusterTasks[0].Type).To(Equal(models.TaskTypeExec))
			Expect(clusterTasks[0].Order).To(Equal(1))
			Expect(clusterTasks[0].Blocking).To(BeTrue())
			Expect(clusterTasks[0].Config.Command).To(Equal("apt-get update && apt-get install -y nginx"))
			Expect(clusterTasks[0].Config.Timeout).To(Equal(300 * time.Second))
			Expect(clusterTasks[0].Config.WorkingDir).To(Equal("/tmp"))

			Expect(clusterTasks[1].Name).To(Equal("Start service"))
			Expect(clusterTasks[1].Type).To(Equal(models.TaskTypeExec))
			Expect(clusterTasks[1].Order).To(Equal(2))
			Expect(clusterTasks[1].Blocking).To(BeFalse())
			Expect(clusterTasks[1].Config.Command).To(Equal("systemctl start nginx"))
			Expect(clusterTasks[1].Config.Timeout).To(Equal(60 * time.Second))
			Expect(clusterTasks[1].Config.WorkingDir).To(Equal(""))
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
		DROP TABLE IF EXISTS template_tasks CASCADE;
		DROP TABLE IF EXISTS templates CASCADE;
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
