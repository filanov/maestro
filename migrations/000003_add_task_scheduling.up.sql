-- Add scheduling columns to tasks table
ALTER TABLE tasks
    ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN schedule_interval VARCHAR(50);

-- Add scheduling columns to template_tasks table (maintain parity)
ALTER TABLE template_tasks
    ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN schedule_interval VARCHAR(50);

-- Add index for efficiently finding scheduled tasks
CREATE INDEX idx_tasks_scheduled
    ON tasks(cluster_id)
    WHERE schedule_enabled = TRUE AND deleted_at IS NULL;

-- Add comments for documentation
COMMENT ON COLUMN tasks.schedule_enabled IS
    'Enable periodic execution of this task';
COMMENT ON COLUMN tasks.schedule_interval IS
    'Interval between executions in Go duration format (e.g., "5m", "1h", "30s"). NULL if not scheduled.';

COMMENT ON COLUMN template_tasks.schedule_enabled IS
    'Enable periodic execution of this task';
COMMENT ON COLUMN template_tasks.schedule_interval IS
    'Interval between executions in Go duration format (e.g., "5m", "1h", "30s"). NULL if not scheduled.';

-- Add comment to task_executions.started_at for scheduling logic
COMMENT ON COLUMN task_executions.started_at IS
    'When this execution started. For scheduled tasks: if (now - started_at) >= interval, task is due for re-execution';
