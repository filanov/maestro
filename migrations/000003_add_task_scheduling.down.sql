-- Remove scheduling index from tasks
DROP INDEX IF EXISTS idx_tasks_scheduled;

-- Remove scheduling columns from tasks
ALTER TABLE tasks
    DROP COLUMN IF EXISTS schedule_interval,
    DROP COLUMN IF EXISTS schedule_enabled;

-- Remove scheduling columns from template_tasks
ALTER TABLE template_tasks
    DROP COLUMN IF EXISTS schedule_interval,
    DROP COLUMN IF EXISTS schedule_enabled;
