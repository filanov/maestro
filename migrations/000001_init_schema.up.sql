-- Clusters
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Agents
CREATE TABLE agents (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive')),
    last_heartbeat TIMESTAMP NOT NULL,
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_reset_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(cluster_id, hostname)
);

CREATE INDEX idx_agents_cluster ON agents(cluster_id);
CREATE INDEX idx_agents_heartbeat ON agents(last_heartbeat) WHERE status = 'active';

-- Tasks
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    "order" INT NOT NULL,
    blocking BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX idx_tasks_cluster_order_active
    ON tasks(cluster_id, "order")
    WHERE deleted_at IS NULL;

CREATE INDEX idx_tasks_cluster_order ON tasks(cluster_id, "order");
CREATE INDEX idx_tasks_deleted_at ON tasks(deleted_at) WHERE deleted_at IS NOT NULL;

-- Task Executions
CREATE TABLE task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    output TEXT,
    exit_code INT,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    error TEXT,
    CONSTRAINT unique_agent_task UNIQUE (agent_id, task_id)
);

CREATE INDEX idx_executions_agent_task ON task_executions(agent_id, task_id);
CREATE INDEX idx_executions_task ON task_executions(task_id);
CREATE INDEX idx_executions_status ON task_executions(status);

-- Debug Tasks
CREATE TABLE debug_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    output TEXT,
    exit_code INT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMP,
    error TEXT
);

CREATE INDEX idx_debug_tasks_agent_status ON debug_tasks(agent_id, status);
CREATE INDEX idx_debug_tasks_created ON debug_tasks(created_at);
