package rest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/api/rest"
	"github.com/filanov/maestro/internal/api/rest/openapi"
	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("REST Server", func() {
	var (
		server    *rest.Server
		mockDB    *db.MockDB
		mockCtrl  *gomock.Controller
		scheduler *engine.Scheduler
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockDB = db.NewMockDB(mockCtrl)
		scheduler = engine.NewScheduler(mockDB)
		server = rest.NewServer(mockDB, scheduler, "localhost", 8080)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("GET /health", func() {
		It("should return ok status", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			w := httptest.NewRecorder()

			server.GetHealth(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var response openapi.HealthResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(*response.Status).To(Equal("ok"))
		})
	})

	Describe("Cluster Operations", func() {
		Describe("POST /clusters", func() {
			It("should create a cluster successfully", func() {
				reqBody := openapi.CreateClusterRequest{
					Name:        "test-cluster",
					Description: ptr("test description"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ interface{}, cluster *models.Cluster) error {
						Expect(cluster.Name).To(Equal("test-cluster"))
						Expect(cluster.Description).To(Equal("test description"))
						cluster.CreatedAt = time.Now()
						cluster.UpdatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateCluster(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
				var response openapi.Cluster
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Name).To(Equal("test-cluster"))
			})

			It("should return 400 when name is missing", func() {
				reqBody := openapi.CreateClusterRequest{
					Name: "",
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateCluster(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 400 when request body is invalid", func() {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader([]byte("invalid json")))
				w := httptest.NewRecorder()

				server.CreateCluster(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 500 when database create fails", func() {
				reqBody := openapi.CreateClusterRequest{
					Name:        "test-cluster",
					Description: ptr("test description"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().CreateCluster(gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateCluster(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /clusters/{id}", func() {
			It("should return a cluster", func() {
				clusterID := uuid.New().String()
				cluster := &models.Cluster{
					ID:          clusterID,
					Name:        "test-cluster",
					Description: "test description",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				mockDB.EXPECT().GetCluster(gomock.Any(), clusterID).Return(cluster, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.GetCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
				var response openapi.Cluster
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Name).To(Equal("test-cluster"))
			})

			It("should return 404 when cluster not found", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().GetCluster(gomock.Any(), clusterID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.GetCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database get fails", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().GetCluster(gomock.Any(), clusterID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.GetCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /clusters", func() {
			It("should list clusters with pagination", func() {
				clusters := []*models.Cluster{
					{
						ID:        uuid.New().String(),
						Name:      "cluster-1",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						ID:        uuid.New().String(),
						Name:      "cluster-2",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}

				mockDB.EXPECT().ListClusters(gomock.Any(), 50, 0).Return(clusters, 2, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
				w := httptest.NewRecorder()

				server.ListClusters(w, req, openapi.ListClustersParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
				var response struct {
					Data   []openapi.Cluster `json:"data"`
					Total  int               `json:"total"`
					Limit  int               `json:"limit"`
					Offset int               `json:"offset"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Data).To(HaveLen(2))
				Expect(response.Total).To(Equal(2))
				Expect(response.Limit).To(Equal(50))
			})

			It("should return 500 when database list fails", func() {
				mockDB.EXPECT().ListClusters(gomock.Any(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
				w := httptest.NewRecorder()

				server.ListClusters(w, req, openapi.ListClustersParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("DELETE /clusters/{id}", func() {
			It("should delete a cluster successfully", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().DeleteCluster(gomock.Any(), clusterID).Return(nil)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.DeleteCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 404 when cluster not found", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().DeleteCluster(gomock.Any(), clusterID).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.DeleteCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database delete fails", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().DeleteCluster(gomock.Any(), clusterID).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/"+clusterID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.DeleteCluster(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Agent Operations", func() {
		Describe("GET /clusters/{id}/agents", func() {
			It("should return 500 when database list fails", func() {
				clusterID := uuid.New().String()

				mockDB.EXPECT().ListAgents(gomock.Any(), clusterID, nil, 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterID+"/agents", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(clusterID)
				server.ListAgentsByCluster(w, req, id, openapi.ListAgentsByClusterParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /agents/{id}", func() {
			It("should return 500 when database get fails", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.GetAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("DELETE /agents/{id}", func() {
			It("should return 500 when database delete fails", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().DeleteAgent(gomock.Any(), agentID).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.DeleteAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /agents/{id}/executions", func() {
			It("should return 500 when database list fails", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().ListExecutions(gomock.Any(), gomock.Any(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID+"/executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.ListAgentExecutions(w, req, openapi.ID(id), openapi.ListAgentExecutionsParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Task Operations", func() {
		Describe("POST /tasks", func() {
			It("should create a task successfully", func() {
				clusterID := uuid.New()
				reqBody := openapi.CreateTaskRequest{
					ClusterId: clusterID,
					Name:      "test-task",
					Type:      "exec",
					Blocking:  ptr(false),
					Config: openapi.TaskConfig{
						Command:        "echo test",
						TimeoutSeconds: ptr(300),
						WorkingDir:     ptr("/tmp"),
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return([]*models.Task{}, nil)
				mockDB.EXPECT().CreateTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ interface{}, task *models.Task) error {
						Expect(task.Name).To(Equal("test-task"))
						Expect(task.Order).To(Equal(0))
						task.CreatedAt = time.Now()
						task.UpdatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTask(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return 400 when name is missing", func() {
				clusterID := uuid.New()
				reqBody := openapi.CreateTaskRequest{
					ClusterId: clusterID,
					Name:      "",
					Type:      "exec",
					Config: openapi.TaskConfig{
						Command: "echo test",
					},
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTask(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 400 when command is missing", func() {
				clusterID := uuid.New()
				reqBody := openapi.CreateTaskRequest{
					ClusterId: clusterID,
					Name:      "test-task",
					Type:      "exec",
					Config: openapi.TaskConfig{
						Command: "",
					},
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTask(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 500 when database get tasks fails", func() {
				clusterID := uuid.New()
				reqBody := openapi.CreateTaskRequest{
					ClusterId: clusterID,
					Name:      "test-task",
					Type:      "exec",
					Config: openapi.TaskConfig{
						Command: "echo test",
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTask(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when database create task fails", func() {
				clusterID := uuid.New()
				reqBody := openapi.CreateTaskRequest{
					ClusterId: clusterID,
					Name:      "test-task",
					Type:      "exec",
					Config: openapi.TaskConfig{
						Command: "echo test",
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return([]*models.Task{}, nil)
				mockDB.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTask(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("PUT /tasks/{id}", func() {
			It("should update a task successfully", func() {
				taskID := uuid.New().String()
				reqBody := openapi.UpdateTaskRequest{
					Name:     ptr("updated-task"),
					Blocking: ptr(true),
				}
				body, _ := json.Marshal(reqBody)

				updatedTask := &models.Task{
					ID:        taskID,
					Name:      "updated-task",
					Blocking:  true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				mockDB.EXPECT().UpdateTask(gomock.Any(), taskID, gomock.Any()).Return(nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), taskID).Return(nil)
				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(updatedTask, nil)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+taskID, bytes.NewReader(body))
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.UpdateTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
				var response openapi.Task
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Name).To(Equal("updated-task"))
			})

			It("should return 404 when task not found", func() {
				taskID := uuid.New().String()
				reqBody := openapi.UpdateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTask(gomock.Any(), taskID, gomock.Any()).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+taskID, bytes.NewReader(body))
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.UpdateTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database update fails", func() {
				taskID := uuid.New().String()
				reqBody := openapi.UpdateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTask(gomock.Any(), taskID, gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+taskID, bytes.NewReader(body))
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.UpdateTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when get updated task fails", func() {
				taskID := uuid.New().String()
				reqBody := openapi.UpdateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTask(gomock.Any(), taskID, gomock.Any()).Return(nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), taskID).Return(nil)
				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+taskID, bytes.NewReader(body))
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.UpdateTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /tasks/reorder", func() {
			It("should reorder tasks successfully", func() {
				clusterID := uuid.New()
				task1ID := uuid.New()
				task2ID := uuid.New()

				reqBody := openapi.ReorderTasksRequest{
					ClusterId: clusterID,
					TaskIds:   []uuid.UUID{task2ID, task1ID},
				}
				body, _ := json.Marshal(reqBody)

				existingTasks := []*models.Task{
					{ID: task1ID.String(), Order: 0},
					{ID: task2ID.String(), Order: 1},
				}

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return(existingTasks, nil)
				mockDB.EXPECT().ReorderTasks(gomock.Any(), clusterID.String(), []string{task2ID.String(), task1ID.String()}).Return(nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), task1ID.String()).Return(nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), task2ID.String()).Return(nil)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 400 when task_ids is empty", func() {
				clusterID := uuid.New()
				reqBody := openapi.ReorderTasksRequest{
					ClusterId: clusterID,
					TaskIds:   []uuid.UUID{},
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 500 when database get tasks fails", func() {
				clusterID := uuid.New()
				task1ID := uuid.New()
				reqBody := openapi.ReorderTasksRequest{
					ClusterId: clusterID,
					TaskIds:   []uuid.UUID{task1ID},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when database reorder fails", func() {
				clusterID := uuid.New()
				task1ID := uuid.New()
				reqBody := openapi.ReorderTasksRequest{
					ClusterId: clusterID,
					TaskIds:   []uuid.UUID{task1ID},
				}
				body, _ := json.Marshal(reqBody)

				existingTasks := []*models.Task{
					{ID: task1ID.String(), Order: 0},
				}

				mockDB.EXPECT().GetTasksForCluster(gomock.Any(), clusterID.String()).Return(existingTasks, nil)
				mockDB.EXPECT().ReorderTasks(gomock.Any(), clusterID.String(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /tasks", func() {
			It("should return 500 when database list fails", func() {
				mockDB.EXPECT().ListTasks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
				w := httptest.NewRecorder()

				server.ListTasks(w, req, openapi.ListTasksParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("DELETE /tasks/{id}", func() {
			It("should return 500 when database delete fails", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().DeleteTask(gomock.Any(), taskID).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+taskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.DeleteTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /tasks/{id}/reset-executions", func() {
			It("should return 500 when database get task fails", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/reset-executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.ResetTaskExecutions(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when database reset executions fails", func() {
				taskID := uuid.New().String()
				task := &models.Task{
					ID:   taskID,
					Name: "test-task",
				}

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(task, nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), taskID).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/reset-executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.ResetTaskExecutions(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Execution Operations", func() {
		Describe("GET /executions", func() {
			It("should return 500 when database list fails", func() {
				mockDB.EXPECT().ListExecutions(gomock.Any(), gomock.Any(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", nil)
				w := httptest.NewRecorder()

				server.ListExecutions(w, req, openapi.ListExecutionsParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /executions/{id}", func() {
			It("should return 500 when database get fails", func() {
				executionID := uuid.New().String()

				mockDB.EXPECT().GetExecution(gomock.Any(), executionID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+executionID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(executionID)
				server.GetExecution(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Template Operations", func() {
		Describe("POST /templates", func() {
			It("should create a template successfully", func() {
				reqBody := openapi.CreateTemplateRequest{
					Name:        "test-template",
					Description: ptr("test description"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().CreateTemplate(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ interface{}, template *models.Template) error {
						Expect(template.Name).To(Equal("test-template"))
						template.CreatedAt = time.Now()
						template.UpdatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTemplate(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return 500 when database create fails", func() {
				reqBody := openapi.CreateTemplateRequest{
					Name:        "test-template",
					Description: ptr("test description"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().CreateTemplate(gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTemplate(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /templates", func() {
			It("should return 500 when database list fails", func() {
				mockDB.EXPECT().ListTemplates(gomock.Any(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
				w := httptest.NewRecorder()

				server.ListTemplates(w, req, openapi.ListTemplatesParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /templates/{id}", func() {
			It("should return a template", func() {
				templateID := uuid.New()
				template := &models.Template{
					ID:          templateID.String(),
					Name:        "test-template",
					Description: "test description",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				mockDB.EXPECT().GetTemplate(gomock.Any(), templateID.String()).Return(template, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID.String(), nil)
				w := httptest.NewRecorder()

				server.GetTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when template not found", func() {
				templateID := uuid.New()

				mockDB.EXPECT().GetTemplate(gomock.Any(), templateID.String()).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID.String(), nil)
				w := httptest.NewRecorder()

				server.GetTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database get fails", func() {
				templateID := uuid.New()

				mockDB.EXPECT().GetTemplate(gomock.Any(), templateID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID.String(), nil)
				w := httptest.NewRecorder()

				server.GetTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("PUT /templates/{id}", func() {
			It("should return 500 when database update fails", func() {
				templateID := uuid.New()
				reqBody := openapi.UpdateTemplateRequest{
					Name: ptr("updated-template"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTemplate(gomock.Any(), templateID.String(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/"+templateID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when get updated template fails", func() {
				templateID := uuid.New()
				reqBody := openapi.UpdateTemplateRequest{
					Name: ptr("updated-template"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTemplate(gomock.Any(), templateID.String(), gomock.Any()).Return(nil)
				mockDB.EXPECT().GetTemplate(gomock.Any(), templateID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/"+templateID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("DELETE /templates/{id}", func() {
			It("should return 500 when database delete fails", func() {
				templateID := uuid.New()

				mockDB.EXPECT().DeleteTemplate(gomock.Any(), templateID.String()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/"+templateID.String(), nil)
				w := httptest.NewRecorder()

				server.DeleteTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /clusters/{id}/import-template", func() {
			It("should import template successfully", func() {
				clusterID := uuid.New()
				templateID := uuid.New()

				reqBody := openapi.ImportTemplateRequest{
					TemplateId: templateID,
				}
				body, _ := json.Marshal(reqBody)

				templateTasks := []*models.TemplateTask{
					{ID: uuid.New().String(), Name: "task1"},
					{ID: uuid.New().String(), Name: "task2"},
				}

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return(templateTasks, nil)
				mockDB.EXPECT().ImportTemplateToCluster(gomock.Any(), clusterID.String(), templateID.String()).Return(nil)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/import-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ImportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusOK))
				var response openapi.ImportTemplateResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.TasksImported).To(Equal(2))
			})

			It("should return 404 when template not found", func() {
				clusterID := uuid.New()
				templateID := uuid.New()

				reqBody := openapi.ImportTemplateRequest{
					TemplateId: templateID,
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/import-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ImportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when get template tasks fails", func() {
				clusterID := uuid.New()
				templateID := uuid.New()

				reqBody := openapi.ImportTemplateRequest{
					TemplateId: templateID,
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/import-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ImportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when database import fails", func() {
				clusterID := uuid.New()
				templateID := uuid.New()

				reqBody := openapi.ImportTemplateRequest{
					TemplateId: templateID,
				}
				body, _ := json.Marshal(reqBody)

				templateTasks := []*models.TemplateTask{
					{ID: uuid.New().String(), Name: "task1"},
				}

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return(templateTasks, nil)
				mockDB.EXPECT().ImportTemplateToCluster(gomock.Any(), clusterID.String(), templateID.String()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/import-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ImportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /clusters/{id}/export-template", func() {
			It("should return 500 when database export fails", func() {
				clusterID := uuid.New()
				reqBody := openapi.ExportTemplateRequest{
					TemplateName: "exported-template",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().ExportClusterToTemplate(gomock.Any(), clusterID.String(), gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/export-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ExportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Debug Task Operations", func() {
		Describe("POST /debug-tasks", func() {
			It("should create a debug task successfully", func() {
				agentID := uuid.New()
				agent := &models.Agent{
					ID:        agentID.String(),
					ClusterID: uuid.New().String(),
				}

				reqBody := openapi.CreateDebugTaskRequest{
					AgentId: agentID,
					Command: "ps aux",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID.String()).Return(agent, nil)
				mockDB.EXPECT().CreateDebugTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ interface{}, task *models.DebugTask) error {
						Expect(task.Command).To(Equal("ps aux"))
						Expect(task.Status).To(Equal(models.ExecutionStatusPending))
						task.CreatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/debug-tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateDebugTask(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return 400 when command is missing", func() {
				agentID := uuid.New()
				reqBody := openapi.CreateDebugTaskRequest{
					AgentId: agentID,
					Command: "",
				}
				body, _ := json.Marshal(reqBody)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/debug-tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateDebugTask(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 400 when agent not found", func() {
				agentID := uuid.New()
				reqBody := openapi.CreateDebugTaskRequest{
					AgentId: agentID,
					Command: "ps aux",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID.String()).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/debug-tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateDebugTask(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("GET /debug-tasks/{id}", func() {
			It("should return a debug task", func() {
				debugTaskID := uuid.New().String()
				debugTask := &models.DebugTask{
					ID:        debugTaskID,
					AgentID:   uuid.New().String(),
					ClusterID: uuid.New().String(),
					Command:   "ps aux",
					Status:    models.ExecutionStatusSuccess,
					Output:    "process list",
					CreatedAt: time.Now(),
				}

				mockDB.EXPECT().GetDebugTask(gomock.Any(), debugTaskID).Return(debugTask, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/debug-tasks/"+debugTaskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(debugTaskID)
				server.GetDebugTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when debug task not found", func() {
				debugTaskID := uuid.New().String()

				mockDB.EXPECT().GetDebugTask(gomock.Any(), debugTaskID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/debug-tasks/"+debugTaskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(debugTaskID)
				server.GetDebugTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database get fails", func() {
				debugTaskID := uuid.New().String()

				mockDB.EXPECT().GetDebugTask(gomock.Any(), debugTaskID).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/debug-tasks/"+debugTaskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(debugTaskID)
				server.GetDebugTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /debug-tasks - Database Errors", func() {
			It("should return 400 when get agent fails", func() {
				agentID := uuid.New()
				reqBody := openapi.CreateDebugTaskRequest{
					AgentId: agentID,
					Command: "ps aux",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/debug-tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateDebugTask(w, req)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 500 when create debug task fails", func() {
				agentID := uuid.New()
				agent := &models.Agent{
					ID:        agentID.String(),
					ClusterID: uuid.New().String(),
				}
				reqBody := openapi.CreateDebugTaskRequest{
					AgentId: agentID,
					Command: "ps aux",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID.String()).Return(agent, nil)
				mockDB.EXPECT().CreateDebugTask(gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/debug-tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateDebugTask(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("GET /agents/{id}/debug-tasks", func() {
			It("should list debug tasks for agent", func() {
				agentID := uuid.New()
				debugTasks := []*models.DebugTask{
					{
						ID:        uuid.New().String(),
						AgentID:   agentID.String(),
						Command:   "ps aux",
						Status:    models.ExecutionStatusSuccess,
						CreatedAt: time.Now(),
					},
				}

				mockDB.EXPECT().ListDebugTasks(gomock.Any(), agentID.String(), 50, 0).Return(debugTasks, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID.String()+"/debug-tasks", nil)
				w := httptest.NewRecorder()

				server.ListDebugTasksByAgent(w, req, agentID, openapi.ListDebugTasksByAgentParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 500 when database list fails", func() {
				agentID := uuid.New()

				mockDB.EXPECT().ListDebugTasks(gomock.Any(), agentID.String(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID.String()+"/debug-tasks", nil)
				w := httptest.NewRecorder()

				server.ListDebugTasksByAgent(w, req, agentID, openapi.ListDebugTasksByAgentParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Agent Operations", func() {
		Describe("GET /clusters/{id}/agents", func() {
			It("should list agents for cluster", func() {
				clusterID := uuid.New()
				agents := []*models.Agent{
					{
						ID:            uuid.New().String(),
						ClusterID:     clusterID.String(),
						Hostname:      "agent-1",
						Status:        models.AgentStatusActive,
						LastHeartbeat: time.Now(),
					},
					{
						ID:            uuid.New().String(),
						ClusterID:     clusterID.String(),
						Hostname:      "agent-2",
						Status:        models.AgentStatusActive,
						LastHeartbeat: time.Now(),
					},
				}

				mockDB.EXPECT().ListAgents(gomock.Any(), clusterID.String(), nil, 50, 0).Return(agents, 2, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterID.String()+"/agents", nil)
				w := httptest.NewRecorder()

				server.ListAgentsByCluster(w, req, clusterID, openapi.ListAgentsByClusterParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("GET /agents/{id}", func() {
			It("should return an agent", func() {
				agentID := uuid.New().String()
				agent := &models.Agent{
					ID:            agentID,
					ClusterID:     uuid.New().String(),
					Hostname:      "agent-1",
					Status:        models.AgentStatusActive,
					LastHeartbeat: time.Now(),
				}

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID).Return(agent, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.GetAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when agent not found", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().GetAgent(gomock.Any(), agentID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.GetAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})

		Describe("DELETE /agents/{id}", func() {
			It("should delete an agent successfully", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().DeleteAgent(gomock.Any(), agentID).Return(nil)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.DeleteAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 404 when agent not found", func() {
				agentID := uuid.New().String()

				mockDB.EXPECT().DeleteAgent(gomock.Any(), agentID).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+agentID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.DeleteAgent(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})

		Describe("GET /agents/{id}/executions", func() {
			It("should list executions for agent", func() {
				agentID := uuid.New().String()
				executions := []*models.TaskExecution{
					{
						ID:        uuid.New().String(),
						AgentID:   agentID,
						TaskID:    uuid.New().String(),
						Status:    models.ExecutionStatusSuccess,
						StartedAt: time.Now(),
					},
				}

				mockDB.EXPECT().ListExecutions(gomock.Any(), gomock.Any(), 50, 0).Return(executions, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID+"/executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(agentID)
				server.ListAgentExecutions(w, req, openapi.ID(id), openapi.ListAgentExecutionsParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})
	})

	Describe("Additional Task Operations", func() {
		Describe("GET /tasks/{id}", func() {
			It("should return a task", func() {
				taskID := uuid.New().String()
				task := &models.Task{
					ID:        taskID,
					ClusterID: uuid.New().String(),
					Name:      "test-task",
					Type:      models.TaskTypeExec,
					Order:     0,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(task, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.GetTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when task not found", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.GetTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})

		Describe("DELETE /tasks/{id}", func() {
			It("should delete a task successfully", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().DeleteTask(gomock.Any(), taskID).Return(nil)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+taskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.DeleteTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 404 when task not found", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().DeleteTask(gomock.Any(), taskID).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+taskID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.DeleteTask(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})

		Describe("GET /tasks", func() {
			It("should list tasks for cluster", func() {
				clusterID := uuid.New()
				tasks := []*models.Task{
					{
						ID:        uuid.New().String(),
						ClusterID: clusterID.String(),
						Name:      "task-1",
						Order:     0,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}

				mockDB.EXPECT().ListTasks(gomock.Any(), clusterID.String(), false, 50, 0).Return(tasks, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks?cluster_id="+clusterID.String(), nil)
				w := httptest.NewRecorder()

				server.ListTasks(w, req, openapi.ListTasksParams{ClusterId: clusterID})

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("POST /tasks/{id}/reset-executions", func() {
			It("should reset task executions successfully", func() {
				taskID := uuid.New().String()
				task := &models.Task{
					ID:        taskID,
					ClusterID: uuid.New().String(),
					Name:      "test-task",
				}

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(task, nil)
				mockDB.EXPECT().ResetExecutionsForTask(gomock.Any(), taskID).Return(nil)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/reset-executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.ResetTaskExecutions(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 404 when task not found", func() {
				taskID := uuid.New().String()

				mockDB.EXPECT().GetTask(gomock.Any(), taskID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/reset-executions", nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(taskID)
				server.ResetTaskExecutions(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("Execution Operations", func() {
		Describe("GET /executions", func() {
			It("should list executions with filters", func() {
				executions := []*models.TaskExecution{
					{
						ID:        uuid.New().String(),
						AgentID:   uuid.New().String(),
						TaskID:    uuid.New().String(),
						Status:    models.ExecutionStatusSuccess,
						StartedAt: time.Now(),
					},
				}

				mockDB.EXPECT().ListExecutions(gomock.Any(), gomock.Any(), 50, 0).Return(executions, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", nil)
				w := httptest.NewRecorder()

				server.ListExecutions(w, req, openapi.ListExecutionsParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("GET /executions/{id}", func() {
			It("should return an execution", func() {
				executionID := uuid.New().String()
				execution := &models.TaskExecution{
					ID:        executionID,
					AgentID:   uuid.New().String(),
					TaskID:    uuid.New().String(),
					Status:    models.ExecutionStatusSuccess,
					Output:    "task output",
					StartedAt: time.Now(),
				}

				mockDB.EXPECT().GetExecution(gomock.Any(), executionID).Return(execution, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+executionID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(executionID)
				server.GetExecution(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when execution not found", func() {
				executionID := uuid.New().String()

				mockDB.EXPECT().GetExecution(gomock.Any(), executionID).Return(nil, db.ErrNotFound)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+executionID, nil)
				w := httptest.NewRecorder()

				id, _ := uuid.Parse(executionID)
				server.GetExecution(w, req, openapi.ID(id))

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("Additional Template Operations", func() {
		Describe("GET /templates", func() {
			It("should list templates with pagination", func() {
				templates := []*models.Template{
					{
						ID:          uuid.New().String(),
						Name:        "template-1",
						Description: "test template",
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				}

				mockDB.EXPECT().ListTemplates(gomock.Any(), 50, 0).Return(templates, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
				w := httptest.NewRecorder()

				server.ListTemplates(w, req, openapi.ListTemplatesParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("PUT /templates/{id}", func() {
			It("should update a template successfully", func() {
				templateID := uuid.New()
				reqBody := openapi.UpdateTemplateRequest{
					Name:        ptr("updated-template"),
					Description: ptr("updated description"),
				}
				body, _ := json.Marshal(reqBody)

				updatedTemplate := &models.Template{
					ID:          templateID.String(),
					Name:        "updated-template",
					Description: "updated description",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				mockDB.EXPECT().UpdateTemplate(gomock.Any(), templateID.String(), gomock.Any()).Return(nil)
				mockDB.EXPECT().GetTemplate(gomock.Any(), templateID.String()).Return(updatedTemplate, nil)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/templates/"+templateID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		Describe("DELETE /templates/{id}", func() {
			It("should delete a template successfully", func() {
				templateID := uuid.New()

				mockDB.EXPECT().DeleteTemplate(gomock.Any(), templateID.String()).Return(nil)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/"+templateID.String(), nil)
				w := httptest.NewRecorder()

				server.DeleteTemplate(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})
		})

		Describe("POST /clusters/{id}/export-template", func() {
			It("should export cluster to template successfully", func() {
				clusterID := uuid.New()
				reqBody := openapi.ExportTemplateRequest{
					TemplateName:        "exported-template",
					TemplateDescription: ptr("exported from cluster"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().ExportClusterToTemplate(gomock.Any(), clusterID.String(), gomock.Any(), nil).DoAndReturn(
					func(_ interface{}, _ string, template *models.Template, _ interface{}) error {
						template.ID = uuid.New().String()
						template.CreatedAt = time.Now()
						template.UpdatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/export-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ExportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return 404 when cluster not found", func() {
				clusterID := uuid.New()
				reqBody := openapi.ExportTemplateRequest{
					TemplateName: "exported-template",
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().ExportClusterToTemplate(gomock.Any(), clusterID.String(), gomock.Any(), nil).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/"+clusterID.String()+"/export-template", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ExportTemplate(w, req, clusterID)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("Template Task Operations", func() {
		Describe("GET /templates/{id}/tasks", func() {
			It("should list template tasks", func() {
				templateID := uuid.New()
				tasks := []*models.TemplateTask{
					{
						ID:         uuid.New().String(),
						TemplateID: templateID.String(),
						Name:       "task-1",
						Order:      0,
						CreatedAt:  time.Now(),
						UpdatedAt:  time.Now(),
					},
				}

				mockDB.EXPECT().ListTemplateTasks(gomock.Any(), templateID.String(), 50, 0).Return(tasks, 1, nil)

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID.String()+"/tasks", nil)
				w := httptest.NewRecorder()

				server.ListTemplateTasks(w, req, templateID, openapi.ListTemplateTasksParams{})

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 500 when database list fails", func() {
				templateID := uuid.New()

				mockDB.EXPECT().ListTemplateTasks(gomock.Any(), templateID.String(), 50, 0).Return(nil, 0, errors.New("database error"))

				req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID.String()+"/tasks", nil)
				w := httptest.NewRecorder()

				server.ListTemplateTasks(w, req, templateID, openapi.ListTemplateTasksParams{})

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /templates/{id}/tasks", func() {
			It("should create a template task successfully", func() {
				templateID := uuid.New()
				reqBody := openapi.CreateTemplateTaskRequest{
					Name:     "test-task",
					Type:     "exec",
					Blocking: ptr(false),
					Config: openapi.TaskConfig{
						Command:        "echo test",
						TimeoutSeconds: ptr(300),
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return([]*models.TemplateTask{}, nil)
				mockDB.EXPECT().CreateTemplateTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ interface{}, task *models.TemplateTask) error {
						task.ID = uuid.New().String()
						task.CreatedAt = time.Now()
						task.UpdatedAt = time.Now()
						return nil
					})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/templates/"+templateID.String()+"/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTemplateTask(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return 500 when database get template tasks fails", func() {
				templateID := uuid.New()
				reqBody := openapi.CreateTemplateTaskRequest{
					Name: "test-task",
					Type: "exec",
					Config: openapi.TaskConfig{
						Command: "echo test",
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/templates/"+templateID.String()+"/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTemplateTask(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when database create template task fails", func() {
				templateID := uuid.New()
				reqBody := openapi.CreateTemplateTaskRequest{
					Name: "test-task",
					Type: "exec",
					Config: openapi.TaskConfig{
						Command: "echo test",
					},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().GetTemplateTasksForTemplate(gomock.Any(), templateID.String()).Return([]*models.TemplateTask{}, nil)
				mockDB.EXPECT().CreateTemplateTask(gomock.Any(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/templates/"+templateID.String()+"/tasks", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.CreateTemplateTask(w, req, templateID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("PUT /template-tasks/{id}", func() {
			It("should update a template task successfully", func() {
				taskID := uuid.New()
				reqBody := openapi.UpdateTemplateTaskRequest{
					Name:     ptr("updated-task"),
					Blocking: ptr(true),
				}
				body, _ := json.Marshal(reqBody)

				updatedTask := &models.TemplateTask{
					ID:        taskID.String(),
					Name:      "updated-task",
					Blocking:  true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				mockDB.EXPECT().UpdateTemplateTask(gomock.Any(), taskID.String(), gomock.Any()).Return(nil)
				mockDB.EXPECT().GetTemplateTask(gomock.Any(), taskID.String()).Return(updatedTask, nil)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/template-tasks/"+taskID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 404 when template task not found", func() {
				taskID := uuid.New()
				reqBody := openapi.UpdateTemplateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTemplateTask(gomock.Any(), taskID.String(), gomock.Any()).Return(db.ErrNotFound)

				req := httptest.NewRequest(http.MethodPut, "/api/v1/template-tasks/"+taskID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return 500 when database update fails", func() {
				taskID := uuid.New()
				reqBody := openapi.UpdateTemplateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTemplateTask(gomock.Any(), taskID.String(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/template-tasks/"+taskID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 500 when get updated template task fails", func() {
				taskID := uuid.New()
				reqBody := openapi.UpdateTemplateTaskRequest{
					Name: ptr("updated-task"),
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().UpdateTemplateTask(gomock.Any(), taskID.String(), gomock.Any()).Return(nil)
				mockDB.EXPECT().GetTemplateTask(gomock.Any(), taskID.String()).Return(nil, errors.New("database error"))

				req := httptest.NewRequest(http.MethodPut, "/api/v1/template-tasks/"+taskID.String(), bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.UpdateTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("DELETE /template-tasks/{id}", func() {
			It("should delete a template task successfully", func() {
				taskID := uuid.New()

				mockDB.EXPECT().DeleteTemplateTask(gomock.Any(), taskID.String()).Return(nil)

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/template-tasks/"+taskID.String(), nil)
				w := httptest.NewRecorder()

				server.DeleteTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})

			It("should return 500 when database delete fails", func() {
				taskID := uuid.New()

				mockDB.EXPECT().DeleteTemplateTask(gomock.Any(), taskID.String()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodDelete, "/api/v1/template-tasks/"+taskID.String(), nil)
				w := httptest.NewRecorder()

				server.DeleteTemplateTask(w, req, taskID)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Describe("POST /template-tasks/reorder", func() {
			It("should reorder template tasks successfully", func() {
				templateID := uuid.New()
				task1ID := uuid.New()
				task2ID := uuid.New()

				reqBody := openapi.ReorderTemplateTasksRequest{
					TemplateId: templateID,
					TaskIds:    []uuid.UUID{task2ID, task1ID},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().ReorderTemplateTasks(gomock.Any(), templateID.String(), []string{task2ID.String(), task1ID.String()}).Return(nil)

				req := httptest.NewRequest(http.MethodPost, "/api/v1/template-tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTemplateTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return 500 when database reorder fails", func() {
				templateID := uuid.New()
				task1ID := uuid.New()

				reqBody := openapi.ReorderTemplateTasksRequest{
					TemplateId: templateID,
					TaskIds:    []uuid.UUID{task1ID},
				}
				body, _ := json.Marshal(reqBody)

				mockDB.EXPECT().ReorderTemplateTasks(gomock.Any(), templateID.String(), gomock.Any()).Return(errors.New("database error"))

				req := httptest.NewRequest(http.MethodPost, "/api/v1/template-tasks/reorder", bytes.NewReader(body))
				w := httptest.NewRecorder()

				server.ReorderTemplateTasks(w, req)

				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})

func ptr[T any](v T) *T {
	return &v
}
