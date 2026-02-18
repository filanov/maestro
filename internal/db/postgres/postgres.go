package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

func New(dbURL string, maxOpenConns, maxIdleConns int, connMaxLifetime, connMaxIdleTime time.Duration) (*DB, error) {
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetConnMaxLifetime(connMaxLifetime)
	conn.SetConnMaxIdleTime(connMaxIdleTime)

	if err := conn.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) CreateCluster(ctx context.Context, cluster *models.Cluster) error {
	query := `
		INSERT INTO clusters (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	cluster.ID = uuid.New().String()
	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()

	_, err := d.conn.ExecContext(ctx, query, cluster.ID, cluster.Name, cluster.Description, cluster.CreatedAt, cluster.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}
	return nil
}

func (d *DB) GetCluster(ctx context.Context, id string) (*models.Cluster, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM clusters
		WHERE id = $1
	`
	cluster := &models.Cluster{}
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&cluster.ID, &cluster.Name, &cluster.Description, &cluster.CreatedAt, &cluster.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}
	return cluster, nil
}

func (d *DB) ListClusters(ctx context.Context, limit, offset int) ([]*models.Cluster, int, error) {
	countQuery := `SELECT COUNT(*) FROM clusters`
	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count clusters: %w", err)
	}

	query := `
		SELECT id, name, description, created_at, updated_at
		FROM clusters
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := d.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	clusters := make([]*models.Cluster, 0)
	for rows.Next() {
		cluster := &models.Cluster{}
		if err := rows.Scan(&cluster.ID, &cluster.Name, &cluster.Description, &cluster.CreatedAt, &cluster.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, cluster)
	}
	return clusters, total, nil
}

func (d *DB) DeleteCluster(ctx context.Context, id string) error {
	query := `DELETE FROM clusters WHERE id = $1`
	result, err := d.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (d *DB) CreateAgent(ctx context.Context, agent *models.Agent) error {
	query := `
		INSERT INTO agents (id, cluster_id, hostname, status, last_heartbeat, registered_at, last_reset_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := d.conn.ExecContext(ctx, query,
		agent.ID, agent.ClusterID, agent.Hostname, agent.Status,
		agent.LastHeartbeat, agent.RegisteredAt, agent.LastResetAt,
	)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	return nil
}

func (d *DB) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	query := `
		SELECT id, cluster_id, hostname, status, last_heartbeat, registered_at, last_reset_at
		FROM agents
		WHERE id = $1
	`
	agent := &models.Agent{}
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&agent.ID, &agent.ClusterID, &agent.Hostname, &agent.Status,
		&agent.LastHeartbeat, &agent.RegisteredAt, &agent.LastResetAt,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return agent, nil
}

func (d *DB) UpdateAgent(ctx context.Context, id string, update *db.AgentUpdate) error {
	query := `
		UPDATE agents
		SET status = COALESCE($2, status),
		    last_heartbeat = COALESCE($3, last_heartbeat),
		    last_reset_at = COALESCE($4, last_reset_at)
		WHERE id = $1
	`
	_, err := d.conn.ExecContext(ctx, query, id, update.Status, update.LastHeartbeat, update.LastResetAt)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	return nil
}

func (d *DB) ListAgents(ctx context.Context, clusterID string, status *models.AgentStatus, limit, offset int) ([]*models.Agent, int, error) {
	countQuery := `SELECT COUNT(*) FROM agents WHERE cluster_id = $1`
	args := []interface{}{clusterID}
	if status != nil {
		countQuery += ` AND status = $2`
		args = append(args, *status)
	}

	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count agents: %w", err)
	}

	query := `
		SELECT id, cluster_id, hostname, status, last_heartbeat, registered_at, last_reset_at
		FROM agents
		WHERE cluster_id = $1
	`
	args = []interface{}{clusterID}
	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}
	query += ` ORDER BY registered_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	agents := make([]*models.Agent, 0)
	for rows.Next() {
		agent := &models.Agent{}
		if err := rows.Scan(&agent.ID, &agent.ClusterID, &agent.Hostname, &agent.Status,
			&agent.LastHeartbeat, &agent.RegisteredAt, &agent.LastResetAt); err != nil {
			return nil, 0, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, agent)
	}
	return agents, total, nil
}

func (d *DB) DeleteAgent(ctx context.Context, id string) error {
	query := `DELETE FROM agents WHERE id = $1`
	result, err := d.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (d *DB) FindAgentsWithHeartbeatBefore(ctx context.Context, threshold time.Time) ([]*models.Agent, error) {
	query := `
		SELECT id, cluster_id, hostname, status, last_heartbeat, registered_at, last_reset_at
		FROM agents
		WHERE status = 'active' AND last_heartbeat < $1
	`
	rows, err := d.conn.QueryContext(ctx, query, threshold)
	if err != nil {
		return nil, fmt.Errorf("find agents with old heartbeat: %w", err)
	}
	defer rows.Close()

	agents := make([]*models.Agent, 0)
	for rows.Next() {
		agent := &models.Agent{}
		if err := rows.Scan(&agent.ID, &agent.ClusterID, &agent.Hostname, &agent.Status,
			&agent.LastHeartbeat, &agent.RegisteredAt, &agent.LastResetAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (d *DB) UpdateAgentStatus(ctx context.Context, id string, status models.AgentStatus) error {
	query := `UPDATE agents SET status = $2 WHERE id = $1`
	_, err := d.conn.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}
	return nil
}

func (d *DB) CreateTask(ctx context.Context, task *models.Task) error {
	maxOrderQuery := `
		SELECT COALESCE(MAX("order"), 0) FROM tasks
		WHERE cluster_id = $1 AND deleted_at IS NULL
	`
	var maxOrder int
	if err := d.conn.QueryRowContext(ctx, maxOrderQuery, task.ClusterID).Scan(&maxOrder); err != nil {
		return fmt.Errorf("get max order: %w", err)
	}

	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var scheduleInterval *string
	if task.ScheduleEnabled && task.ScheduleInterval > 0 {
		interval := task.ScheduleInterval.String()
		scheduleInterval = &interval
	}

	query := `
		INSERT INTO tasks (id, cluster_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	task.ID = uuid.New().String()
	task.Order = maxOrder + 1
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	_, err = d.conn.ExecContext(ctx, query,
		task.ID, task.ClusterID, task.Name, task.Type, task.Order,
		task.Blocking, configJSON, task.ScheduleEnabled, scheduleInterval, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (d *DB) GetTask(ctx context.Context, id string) (*models.Task, error) {
	query := `
		SELECT id, cluster_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at, deleted_at
		FROM tasks
		WHERE id = $1
	`
	task := &models.Task{}
	var configJSON []byte
	var scheduleInterval sql.NullString
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.ClusterID, &task.Name, &task.Type, &task.Order,
		&task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	if err := json.Unmarshal(configJSON, &task.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if scheduleInterval.Valid {
		duration, err := time.ParseDuration(scheduleInterval.String)
		if err != nil {
			return nil, fmt.Errorf("parse schedule interval: %w", err)
		}
		task.ScheduleInterval = duration
	}

	return task, nil
}

func (d *DB) UpdateTask(ctx context.Context, id string, update *db.TaskUpdate) error {
	task, err := d.GetTask(ctx, id)
	if err != nil {
		return err
	}

	if update.Name != nil {
		task.Name = *update.Name
	}
	if update.Type != nil {
		task.Type = *update.Type
	}
	if update.Blocking != nil {
		task.Blocking = *update.Blocking
	}
	if update.Config != nil {
		task.Config = *update.Config
	}
	if update.ScheduleEnabled != nil {
		task.ScheduleEnabled = *update.ScheduleEnabled
	}
	if update.ScheduleInterval != nil {
		task.ScheduleInterval = *update.ScheduleInterval
	}
	task.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var scheduleInterval *string
	if task.ScheduleEnabled && task.ScheduleInterval > 0 {
		interval := task.ScheduleInterval.String()
		scheduleInterval = &interval
	}

	query := `
		UPDATE tasks
		SET name = $2, type = $3, blocking = $4, config = $5, schedule_enabled = $6, schedule_interval = $7, updated_at = $8
		WHERE id = $1
	`
	_, err = d.conn.ExecContext(ctx, query, id, task.Name, task.Type, task.Blocking, configJSON, task.ScheduleEnabled, scheduleInterval, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (d *DB) DeleteTask(ctx context.Context, id string) error {
	task, err := d.GetTask(ctx, id)
	if err != nil {
		return err
	}

	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	deleteQuery := `UPDATE tasks SET deleted_at = NOW() WHERE id = $1`
	if _, err := tx.ExecContext(ctx, deleteQuery, id); err != nil {
		return fmt.Errorf("soft delete task: %w", err)
	}

	// Get tasks that need reordering (order > deleted task's order)
	getTasksQuery := `
		SELECT id, "order"
		FROM tasks
		WHERE cluster_id = $1 AND "order" > $2 AND deleted_at IS NULL
		ORDER BY "order" ASC
	`
	rows, err := tx.QueryContext(ctx, getTasksQuery, task.ClusterID, task.Order)
	if err != nil {
		return fmt.Errorf("get tasks for reorder: %w", err)
	}

	type taskOrder struct {
		id    string
		order int
	}
	var tasksToReorder []taskOrder
	for rows.Next() {
		var t taskOrder
		if err := rows.Scan(&t.id, &t.order); err != nil {
			rows.Close()
			return fmt.Errorf("scan task: %w", err)
		}
		tasksToReorder = append(tasksToReorder, t)
	}
	rows.Close()

	// Two-phase update to avoid constraint violations
	// First pass: set to high temporary values
	for i, t := range tasksToReorder {
		query := `UPDATE tasks SET "order" = $1 WHERE id = $2`
		if _, err := tx.ExecContext(ctx, query, 1000+i, t.id); err != nil {
			return fmt.Errorf("update task order (temp): %w", err)
		}
	}

	// Second pass: set to final values (original order - 1)
	for _, t := range tasksToReorder {
		newOrder := t.order - 1
		query := `UPDATE tasks SET "order" = $1 WHERE id = $2`
		if _, err := tx.ExecContext(ctx, query, newOrder, t.id); err != nil {
			return fmt.Errorf("update task order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (d *DB) ListTasks(ctx context.Context, clusterID string, includeDeleted bool, limit, offset int) ([]*models.Task, int, error) {
	countQuery := `SELECT COUNT(*) FROM tasks WHERE cluster_id = $1`
	if !includeDeleted {
		countQuery += ` AND deleted_at IS NULL`
	}

	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery, clusterID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	query := `
		SELECT id, cluster_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at, deleted_at
		FROM tasks
		WHERE cluster_id = $1
	`
	if !includeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	query += ` ORDER BY "order" ASC LIMIT $2 OFFSET $3`

	rows, err := d.conn.QueryContext(ctx, query, clusterID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.Task, 0)
	for rows.Next() {
		task := &models.Task{}
		var configJSON []byte
		var scheduleInterval sql.NullString
		if err := rows.Scan(&task.ID, &task.ClusterID, &task.Name, &task.Type, &task.Order,
			&task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		if err := json.Unmarshal(configJSON, &task.Config); err != nil {
			return nil, 0, fmt.Errorf("unmarshal config: %w", err)
		}
		if scheduleInterval.Valid {
			duration, err := time.ParseDuration(scheduleInterval.String)
			if err != nil {
				return nil, 0, fmt.Errorf("parse schedule interval: %w", err)
			}
			task.ScheduleInterval = duration
		}
		tasks = append(tasks, task)
	}
	return tasks, total, nil
}

func (d *DB) GetTasksForCluster(ctx context.Context, clusterID string) ([]*models.Task, error) {
	query := `
		SELECT id, cluster_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at, deleted_at
		FROM tasks
		WHERE cluster_id = $1 AND deleted_at IS NULL
		ORDER BY "order" ASC
	`
	rows, err := d.conn.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("get tasks for cluster: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.Task, 0)
	for rows.Next() {
		task := &models.Task{}
		var configJSON []byte
		var scheduleInterval sql.NullString
		if err := rows.Scan(&task.ID, &task.ClusterID, &task.Name, &task.Type, &task.Order,
			&task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt, &task.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if err := json.Unmarshal(configJSON, &task.Config); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		if scheduleInterval.Valid {
			duration, err := time.ParseDuration(scheduleInterval.String)
			if err != nil {
				return nil, fmt.Errorf("parse schedule interval: %w", err)
			}
			task.ScheduleInterval = duration
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (d *DB) ReorderTasks(ctx context.Context, clusterID string, taskIDs []string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First pass: set all orders to high values to avoid constraint violations
	for i, taskID := range taskIDs {
		query := `UPDATE tasks SET "order" = $1 WHERE id = $2 AND cluster_id = $3 AND deleted_at IS NULL`
		if _, err := tx.ExecContext(ctx, query, 1000+i, taskID, clusterID); err != nil {
			return fmt.Errorf("update task order (temp): %w", err)
		}
	}

	// Second pass: set final order values
	for i, taskID := range taskIDs {
		query := `UPDATE tasks SET "order" = $1 WHERE id = $2 AND cluster_id = $3 AND deleted_at IS NULL`
		if _, err := tx.ExecContext(ctx, query, i+1, taskID, clusterID); err != nil {
			return fmt.Errorf("update task order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (d *DB) CreateExecution(ctx context.Context, execution *models.TaskExecution) error {
	query := `
		INSERT INTO task_executions (id, task_id, agent_id, cluster_id, status, output, exit_code, started_at, completed_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	execution.ID = uuid.New().String()
	_, err := d.conn.ExecContext(ctx, query,
		execution.ID, execution.TaskID, execution.AgentID, execution.ClusterID, execution.Status,
		execution.Output, execution.ExitCode, execution.StartedAt, execution.CompletedAt, execution.Error,
	)
	if err != nil {
		return fmt.Errorf("create execution: %w", err)
	}
	return nil
}

func (d *DB) UpsertExecution(ctx context.Context, execution *models.TaskExecution) error {
	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}
	query := `
		INSERT INTO task_executions (id, agent_id, task_id, cluster_id, status, output, exit_code, started_at, completed_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (agent_id, task_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			output = EXCLUDED.output,
			exit_code = EXCLUDED.exit_code,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			error = EXCLUDED.error
	`
	_, err := d.conn.ExecContext(ctx, query,
		execution.ID, execution.AgentID, execution.TaskID, execution.ClusterID, execution.Status,
		execution.Output, execution.ExitCode, execution.StartedAt, execution.CompletedAt, execution.Error,
	)
	if err != nil {
		return fmt.Errorf("upsert execution: %w", err)
	}
	return nil
}

func (d *DB) GetExecution(ctx context.Context, id string) (*models.TaskExecution, error) {
	query := `
		SELECT id, agent_id, task_id, cluster_id, status, output, exit_code, started_at, completed_at, error
		FROM task_executions
		WHERE id = $1
	`
	execution := &models.TaskExecution{}
	var exitCode sql.NullInt32
	var output sql.NullString
	var completedAt sql.NullTime
	var errorMsg sql.NullString

	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&execution.ID, &execution.AgentID, &execution.TaskID, &execution.ClusterID, &execution.Status,
		&output, &exitCode, &execution.StartedAt, &completedAt, &errorMsg,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}

	if output.Valid {
		execution.Output = output.String
	}
	if exitCode.Valid {
		val := int(exitCode.Int32)
		execution.ExitCode = &val
	}
	if completedAt.Valid {
		execution.CompletedAt = &completedAt.Time
	}
	if errorMsg.Valid {
		execution.Error = errorMsg.String
	}

	return execution, nil
}

func (d *DB) ListExecutions(ctx context.Context, filters db.ExecutionFilters, limit, offset int) ([]*models.TaskExecution, int, error) {
	countQuery := `SELECT COUNT(*) FROM task_executions WHERE 1=1`
	query := `
		SELECT id, agent_id, task_id, cluster_id, status, output, exit_code, started_at, completed_at, error
		FROM task_executions
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.ClusterID != nil {
		countQuery += fmt.Sprintf(` AND cluster_id = $%d`, argIdx)
		query += fmt.Sprintf(` AND cluster_id = $%d`, argIdx)
		args = append(args, *filters.ClusterID)
		argIdx++
	}
	if filters.AgentID != nil {
		countQuery += fmt.Sprintf(` AND agent_id = $%d`, argIdx)
		query += fmt.Sprintf(` AND agent_id = $%d`, argIdx)
		args = append(args, *filters.AgentID)
		argIdx++
	}
	if filters.TaskID != nil {
		countQuery += fmt.Sprintf(` AND task_id = $%d`, argIdx)
		query += fmt.Sprintf(` AND task_id = $%d`, argIdx)
		args = append(args, *filters.TaskID)
		argIdx++
	}
	if filters.Status != nil {
		countQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}

	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count executions: %w", err)
	}

	query += fmt.Sprintf(` ORDER BY started_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	executions := make([]*models.TaskExecution, 0)
	for rows.Next() {
		execution := &models.TaskExecution{}
		var output, errorStr sql.NullString
		var exitCode sql.NullInt64
		var completedAt sql.NullTime

		if err := rows.Scan(&execution.ID, &execution.AgentID, &execution.TaskID, &execution.ClusterID, &execution.Status,
			&output, &exitCode, &execution.StartedAt, &completedAt, &errorStr); err != nil {
			return nil, 0, fmt.Errorf("scan execution: %w", err)
		}

		execution.Output = output.String
		if exitCode.Valid {
			exitCodeInt := int(exitCode.Int64)
			execution.ExitCode = &exitCodeInt
		}
		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}
		execution.Error = errorStr.String

		executions = append(executions, execution)
	}
	return executions, total, nil
}

func (d *DB) GetExecutionsForAgent(ctx context.Context, agentID string) ([]*models.TaskExecution, error) {
	query := `
		SELECT id, agent_id, task_id, cluster_id, status, output, exit_code, started_at, completed_at, error
		FROM task_executions
		WHERE agent_id = $1
	`
	rows, err := d.conn.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("get executions for agent: %w", err)
	}
	defer rows.Close()

	executions := make([]*models.TaskExecution, 0)
	for rows.Next() {
		execution := &models.TaskExecution{}
		var output, errorStr sql.NullString
		var exitCode sql.NullInt64
		var completedAt sql.NullTime

		if err := rows.Scan(&execution.ID, &execution.AgentID, &execution.TaskID, &execution.ClusterID, &execution.Status,
			&output, &exitCode, &execution.StartedAt, &completedAt, &errorStr); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}

		execution.Output = output.String
		if exitCode.Valid {
			exitCodeInt := int(exitCode.Int64)
			execution.ExitCode = &exitCodeInt
		}
		if completedAt.Valid {
			execution.CompletedAt = &completedAt.Time
		}
		execution.Error = errorStr.String

		executions = append(executions, execution)
	}
	return executions, nil
}

func (d *DB) DeleteAllExecutionsForAgent(ctx context.Context, agentID string) error {
	query := `DELETE FROM task_executions WHERE agent_id = $1`
	_, err := d.conn.ExecContext(ctx, query, agentID)
	if err != nil {
		return fmt.Errorf("delete executions for agent: %w", err)
	}
	return nil
}

func (d *DB) FailRunningTasksForAgent(ctx context.Context, agentID string) error {
	query := `
		UPDATE task_executions
		SET status = 'failed',
		    completed_at = NOW(),
		    error = 'Agent became inactive during execution'
		WHERE agent_id = $1 AND status = 'running'
	`
	_, err := d.conn.ExecContext(ctx, query, agentID)
	if err != nil {
		return fmt.Errorf("fail running tasks: %w", err)
	}
	return nil
}

func (d *DB) ResetExecutionsForTask(ctx context.Context, taskID string) error {
	query := `
		UPDATE task_executions
		SET status = 'pending',
		    output = '',
		    exit_code = NULL,
		    error = '',
		    completed_at = NULL
		WHERE task_id = $1
	`
	_, err := d.conn.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("reset executions for task: %w", err)
	}
	return nil
}

func (d *DB) ResetExecutionForAgent(ctx context.Context, taskID, agentID string) error {
	query := `
		UPDATE task_executions
		SET status = 'pending',
		    output = '',
		    exit_code = NULL,
		    error = '',
		    completed_at = NULL
		WHERE task_id = $1 AND agent_id = $2
	`
	_, err := d.conn.ExecContext(ctx, query, taskID, agentID)
	if err != nil {
		return fmt.Errorf("reset execution for agent: %w", err)
	}
	return nil
}

func (d *DB) CreateDebugTask(ctx context.Context, task *models.DebugTask) error {
	query := `
		INSERT INTO debug_tasks (id, cluster_id, agent_id, command, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	task.ID = uuid.New().String()
	task.CreatedAt = time.Now()
	task.Status = models.ExecutionStatusPending

	_, err := d.conn.ExecContext(ctx, query, task.ID, task.ClusterID, task.AgentID, task.Command, task.Status, task.CreatedAt)
	if err != nil {
		return fmt.Errorf("create debug task: %w", err)
	}
	return nil
}

func (d *DB) GetDebugTask(ctx context.Context, id string) (*models.DebugTask, error) {
	query := `
		SELECT id, cluster_id, agent_id, command, status, COALESCE(output, ''), exit_code, created_at, executed_at, COALESCE(error, '')
		FROM debug_tasks
		WHERE id = $1
	`
	task := &models.DebugTask{}
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.ClusterID, &task.AgentID, &task.Command, &task.Status,
		&task.Output, &task.ExitCode, &task.CreatedAt, &task.ExecutedAt, &task.Error,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get debug task: %w", err)
	}
	return task, nil
}

func (d *DB) ListDebugTasks(ctx context.Context, agentID string, limit, offset int) ([]*models.DebugTask, int, error) {
	countQuery := `SELECT COUNT(*) FROM debug_tasks WHERE agent_id = $1`
	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery, agentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count debug tasks: %w", err)
	}

	query := `
		SELECT id, cluster_id, agent_id, command, status, COALESCE(output, ''), exit_code, created_at, executed_at, COALESCE(error, '')
		FROM debug_tasks
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := d.conn.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list debug tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.DebugTask, 0)
	for rows.Next() {
		task := &models.DebugTask{}
		if err := rows.Scan(&task.ID, &task.ClusterID, &task.AgentID, &task.Command, &task.Status,
			&task.Output, &task.ExitCode, &task.CreatedAt, &task.ExecutedAt, &task.Error); err != nil {
			return nil, 0, fmt.Errorf("scan debug task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, total, nil
}

func (d *DB) GetPendingDebugTasksForAgent(ctx context.Context, agentID string) ([]*models.DebugTask, error) {
	query := `
		SELECT id, cluster_id, agent_id, command, status, COALESCE(output, ''), exit_code, created_at, executed_at, COALESCE(error, '')
		FROM debug_tasks
		WHERE agent_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
	`
	rows, err := d.conn.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("get pending debug tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.DebugTask, 0)
	for rows.Next() {
		task := &models.DebugTask{}
		if err := rows.Scan(&task.ID, &task.ClusterID, &task.AgentID, &task.Command, &task.Status,
			&task.Output, &task.ExitCode, &task.CreatedAt, &task.ExecutedAt, &task.Error); err != nil {
			return nil, fmt.Errorf("scan debug task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (d *DB) UpdateDebugTaskExecution(ctx context.Context, id string, status models.ExecutionStatus, output string, exitCode *int, errorMsg string) error {
	query := `
		UPDATE debug_tasks
		SET status = $2, output = $3, exit_code = $4, executed_at = NOW(), error = $5
		WHERE id = $1
	`
	_, err := d.conn.ExecContext(ctx, query, id, status, output, exitCode, errorMsg)
	if err != nil {
		return fmt.Errorf("update debug task execution: %w", err)
	}
	return nil
}

func (d *DB) DeleteCompletedDebugTasksOlderThan(ctx context.Context, threshold time.Time) error {
	query := `
		DELETE FROM debug_tasks
		WHERE status IN ('success', 'failed') AND executed_at < $1
	`
	_, err := d.conn.ExecContext(ctx, query, threshold)
	if err != nil {
		return fmt.Errorf("delete old debug tasks: %w", err)
	}
	return nil
}

func (d *DB) TimeoutPendingDebugTasks(ctx context.Context, threshold time.Time) error {
	query := `
		UPDATE debug_tasks
		SET status = 'failed', error = 'Task timeout - not executed within 1 hour'
		WHERE status = 'pending' AND created_at < $1
	`
	_, err := d.conn.ExecContext(ctx, query, threshold)
	if err != nil {
		return fmt.Errorf("timeout pending debug tasks: %w", err)
	}
	return nil
}

func (d *DB) DeleteExecutionsForDeletedTasksOlderThan(ctx context.Context, threshold time.Time) error {
	query := `
		DELETE FROM task_executions
		WHERE task_id IN (
			SELECT id FROM tasks WHERE deleted_at IS NOT NULL AND deleted_at < $1
		)
	`
	_, err := d.conn.ExecContext(ctx, query, threshold)
	if err != nil {
		return fmt.Errorf("delete executions for deleted tasks: %w", err)
	}
	return nil
}

func (d *DB) DeleteOldExecutionsKeepLastN(ctx context.Context, keepN int) error {
	query := `
		WITH ranked_executions AS (
			SELECT
				te.id,
				ROW_NUMBER() OVER (
					PARTITION BY te.agent_id, te.task_id
					ORDER BY te.started_at DESC
				) as row_num
			FROM task_executions te
			INNER JOIN tasks t ON te.task_id = t.id
			WHERE t.deleted_at IS NULL
		)
		DELETE FROM task_executions
		WHERE id IN (
			SELECT id FROM ranked_executions WHERE row_num > $1
		)
	`
	_, err := d.conn.ExecContext(ctx, query, keepN)
	if err != nil {
		return fmt.Errorf("delete old executions: %w", err)
	}
	return nil
}

func (d *DB) CreateTemplate(ctx context.Context, template *models.Template) error {
	query := `
		INSERT INTO templates (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	template.ID = uuid.New().String()
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	_, err := d.conn.ExecContext(ctx, query, template.ID, template.Name, template.Description, template.CreatedAt, template.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	return nil
}

func (d *DB) GetTemplate(ctx context.Context, id string) (*models.Template, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM templates
		WHERE id = $1
	`
	template := &models.Template{}
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&template.ID, &template.Name, &template.Description, &template.CreatedAt, &template.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	return template, nil
}

func (d *DB) UpdateTemplate(ctx context.Context, id string, update *db.TemplateUpdate) error {
	query := `
		UPDATE templates
		SET name = COALESCE($1, name),
		    description = COALESCE($2, description),
		    updated_at = $3
		WHERE id = $4
	`
	_, err := d.conn.ExecContext(ctx, query, update.Name, update.Description, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	return nil
}

func (d *DB) DeleteTemplate(ctx context.Context, id string) error {
	query := `DELETE FROM templates WHERE id = $1`
	_, err := d.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	return nil
}

func (d *DB) ListTemplates(ctx context.Context, limit, offset int) ([]*models.Template, int, error) {
	countQuery := `SELECT COUNT(*) FROM templates`
	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count templates: %w", err)
	}

	query := `
		SELECT id, name, description, created_at, updated_at
		FROM templates
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := d.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*models.Template, 0)
	for rows.Next() {
		template := &models.Template{}
		if err := rows.Scan(&template.ID, &template.Name, &template.Description, &template.CreatedAt, &template.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate templates: %w", err)
	}
	return templates, total, nil
}

func (d *DB) CreateTemplateTask(ctx context.Context, task *models.TemplateTask) error {
	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var scheduleInterval *string
	if task.ScheduleEnabled && task.ScheduleInterval > 0 {
		interval := task.ScheduleInterval.String()
		scheduleInterval = &interval
	}

	query := `
		INSERT INTO template_tasks (id, template_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	task.ID = uuid.New().String()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	_, err = d.conn.ExecContext(ctx, query,
		task.ID, task.TemplateID, task.Name, task.Type, task.Order, task.Blocking, configJSON, task.ScheduleEnabled, scheduleInterval, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create template task: %w", err)
	}
	return nil
}

func (d *DB) GetTemplateTask(ctx context.Context, id string) (*models.TemplateTask, error) {
	query := `
		SELECT id, template_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at
		FROM template_tasks
		WHERE id = $1
	`
	task := &models.TemplateTask{}
	var configJSON []byte
	var scheduleInterval sql.NullString
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.TemplateID, &task.Name, &task.Type, &task.Order, &task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template task: %w", err)
	}
	if err := json.Unmarshal(configJSON, &task.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if scheduleInterval.Valid {
		duration, err := time.ParseDuration(scheduleInterval.String)
		if err != nil {
			return nil, fmt.Errorf("parse schedule interval: %w", err)
		}
		task.ScheduleInterval = duration
	}
	return task, nil
}

func (d *DB) UpdateTemplateTask(ctx context.Context, id string, update *db.TemplateTaskUpdate) error {
	task, err := d.GetTemplateTask(ctx, id)
	if err != nil {
		return err
	}

	if update.Name != nil {
		task.Name = *update.Name
	}
	if update.Type != nil {
		task.Type = *update.Type
	}
	if update.Blocking != nil {
		task.Blocking = *update.Blocking
	}
	if update.Config != nil {
		task.Config = *update.Config
	}
	if update.ScheduleEnabled != nil {
		task.ScheduleEnabled = *update.ScheduleEnabled
	}
	if update.ScheduleInterval != nil {
		task.ScheduleInterval = *update.ScheduleInterval
	}
	task.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	var scheduleInterval *string
	if task.ScheduleEnabled && task.ScheduleInterval > 0 {
		interval := task.ScheduleInterval.String()
		scheduleInterval = &interval
	}

	query := `
		UPDATE template_tasks
		SET name = $2, type = $3, blocking = $4, config = $5, schedule_enabled = $6, schedule_interval = $7, updated_at = $8
		WHERE id = $1
	`
	_, err = d.conn.ExecContext(ctx, query, id, task.Name, task.Type, task.Blocking, configJSON, task.ScheduleEnabled, scheduleInterval, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update template task: %w", err)
	}
	return nil
}

func (d *DB) ListTemplateTasks(ctx context.Context, templateID string, limit, offset int) ([]*models.TemplateTask, int, error) {
	countQuery := `SELECT COUNT(*) FROM template_tasks WHERE template_id = $1`
	var total int
	if err := d.conn.QueryRowContext(ctx, countQuery, templateID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count template tasks: %w", err)
	}

	query := `
		SELECT id, template_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at
		FROM template_tasks
		WHERE template_id = $1
		ORDER BY "order" ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := d.conn.QueryContext(ctx, query, templateID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list template tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.TemplateTask, 0)
	for rows.Next() {
		task := &models.TemplateTask{}
		var configJSON []byte
		var scheduleInterval sql.NullString
		if err := rows.Scan(&task.ID, &task.TemplateID, &task.Name, &task.Type, &task.Order, &task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan template task: %w", err)
		}
		if err := json.Unmarshal(configJSON, &task.Config); err != nil {
			return nil, 0, fmt.Errorf("unmarshal config: %w", err)
		}
		if scheduleInterval.Valid {
			duration, err := time.ParseDuration(scheduleInterval.String)
			if err != nil {
				return nil, 0, fmt.Errorf("parse schedule interval: %w", err)
			}
			task.ScheduleInterval = duration
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate template tasks: %w", err)
	}
	return tasks, total, nil
}

func (d *DB) GetTemplateTasksForTemplate(ctx context.Context, templateID string) ([]*models.TemplateTask, error) {
	query := `
		SELECT id, template_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at
		FROM template_tasks
		WHERE template_id = $1
		ORDER BY "order" ASC
	`
	rows, err := d.conn.QueryContext(ctx, query, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*models.TemplateTask, 0)
	for rows.Next() {
		task := &models.TemplateTask{}
		var configJSON []byte
		var scheduleInterval sql.NullString
		if err := rows.Scan(&task.ID, &task.TemplateID, &task.Name, &task.Type, &task.Order, &task.Blocking, &configJSON, &task.ScheduleEnabled, &scheduleInterval, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template task: %w", err)
		}
		if err := json.Unmarshal(configJSON, &task.Config); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		if scheduleInterval.Valid {
			duration, err := time.ParseDuration(scheduleInterval.String)
			if err != nil {
				return nil, fmt.Errorf("parse schedule interval: %w", err)
			}
			task.ScheduleInterval = duration
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate template tasks: %w", err)
	}
	return tasks, nil
}

func (d *DB) DeleteTemplateTask(ctx context.Context, id string) error {
	query := `DELETE FROM template_tasks WHERE id = $1`
	_, err := d.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete template task: %w", err)
	}
	return nil
}

func (d *DB) ReorderTemplateTasks(ctx context.Context, templateID string, taskIDs []string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First pass: set all orders to high values to avoid constraint violations
	for i, taskID := range taskIDs {
		query := `UPDATE template_tasks SET "order" = $1 WHERE id = $2 AND template_id = $3`
		if _, err := tx.ExecContext(ctx, query, 1000+i, taskID, templateID); err != nil {
			return fmt.Errorf("update template task order (temp): %w", err)
		}
	}

	// Second pass: set final order values
	for i, taskID := range taskIDs {
		query := `UPDATE template_tasks SET "order" = $1 WHERE id = $2 AND template_id = $3`
		if _, err := tx.ExecContext(ctx, query, i+1, taskID, templateID); err != nil {
			return fmt.Errorf("update template task order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (d *DB) ImportTemplateToCluster(ctx context.Context, clusterID, templateID string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var clusterExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1)`, clusterID).Scan(&clusterExists); err != nil {
		return fmt.Errorf("check cluster exists: %w", err)
	}
	if !clusterExists {
		return db.ErrNotFound
	}

	var templateExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM templates WHERE id = $1)`, templateID).Scan(&templateExists); err != nil {
		return fmt.Errorf("check template exists: %w", err)
	}
	if !templateExists {
		return db.ErrNotFound
	}

	var maxOrder int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX("order"), 0) FROM tasks WHERE cluster_id = $1 AND deleted_at IS NULL`, clusterID).Scan(&maxOrder); err != nil {
		return fmt.Errorf("get max order: %w", err)
	}

	query := `
		SELECT id, name, type, "order", blocking, config, schedule_enabled, schedule_interval
		FROM template_tasks
		WHERE template_id = $1
		ORDER BY "order" ASC
	`
	rows, err := tx.QueryContext(ctx, query, templateID)
	if err != nil {
		return fmt.Errorf("get template tasks: %w", err)
	}

	type templateTaskData struct {
		name             string
		taskType         string
		order            int
		blocking         bool
		configJSON       []byte
		scheduleEnabled  bool
		scheduleInterval sql.NullString
	}
	var templateTasks []templateTaskData

	for rows.Next() {
		var templateTaskID string
		var ttd templateTaskData

		if err := rows.Scan(&templateTaskID, &ttd.name, &ttd.taskType, &ttd.order, &ttd.blocking, &ttd.configJSON, &ttd.scheduleEnabled, &ttd.scheduleInterval); err != nil {
			rows.Close()
			return fmt.Errorf("scan template task: %w", err)
		}
		templateTasks = append(templateTasks, ttd)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate template tasks: %w", err)
	}
	rows.Close()

	insertQuery := `
		INSERT INTO tasks (id, cluster_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	now := time.Now()
	for _, ttd := range templateTasks {
		newTaskID := uuid.New().String()
		newOrder := maxOrder + ttd.order

		var scheduleInterval *string
		if ttd.scheduleInterval.Valid {
			scheduleInterval = &ttd.scheduleInterval.String
		}

		_, err = tx.ExecContext(ctx, insertQuery,
			newTaskID, clusterID, ttd.name, ttd.taskType, newOrder, ttd.blocking, ttd.configJSON, ttd.scheduleEnabled, scheduleInterval, now, now)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
	}

	return tx.Commit()
}

func (d *DB) ExportClusterToTemplate(ctx context.Context, clusterID string, template *models.Template, taskIDs []string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var clusterExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1)`, clusterID).Scan(&clusterExists); err != nil {
		return fmt.Errorf("check cluster exists: %w", err)
	}
	if !clusterExists {
		return db.ErrNotFound
	}

	template.ID = uuid.New().String()
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	insertTemplateQuery := `
		INSERT INTO templates (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.ExecContext(ctx, insertTemplateQuery,
		template.ID, template.Name, template.Description, template.CreatedAt, template.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	query := `
		SELECT id, name, type, "order", blocking, config, schedule_enabled, schedule_interval
		FROM tasks
		WHERE cluster_id = $1 AND deleted_at IS NULL`

	args := []interface{}{clusterID}

	if len(taskIDs) > 0 {
		query += ` AND id = ANY($2)`
		args = append(args, taskIDs)
	}

	query += ` ORDER BY "order" ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("get cluster tasks: %w", err)
	}

	type taskData struct {
		name             string
		taskType         string
		blocking         bool
		configJSON       []byte
		scheduleEnabled  bool
		scheduleInterval sql.NullString
	}
	var tasks []taskData

	for rows.Next() {
		var taskID string
		var order int
		var td taskData

		if err := rows.Scan(&taskID, &td.name, &td.taskType, &order, &td.blocking, &td.configJSON, &td.scheduleEnabled, &td.scheduleInterval); err != nil {
			rows.Close()
			return fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, td)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate tasks: %w", err)
	}
	rows.Close()

	insertTaskQuery := `
		INSERT INTO template_tasks (id, template_id, name, type, "order", blocking, config, schedule_enabled, schedule_interval, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	now := time.Now()
	for i, td := range tasks {
		newTaskID := uuid.New().String()
		newOrder := i + 1

		var scheduleInterval *string
		if td.scheduleInterval.Valid {
			scheduleInterval = &td.scheduleInterval.String
		}

		_, err = tx.ExecContext(ctx, insertTaskQuery,
			newTaskID, template.ID, td.name, td.taskType, newOrder, td.blocking, td.configJSON, td.scheduleEnabled, scheduleInterval, now, now)
		if err != nil {
			return fmt.Errorf("insert template task: %w", err)
		}
	}

	return tx.Commit()
}
