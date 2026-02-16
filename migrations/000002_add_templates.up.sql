-- Templates table (global, not tied to clusters)
CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_name ON templates(name);

-- Template Tasks table (normalized, similar to tasks table)
CREATE TABLE template_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    "order" INT NOT NULL,
    blocking BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_template_order UNIQUE (template_id, "order")
);

CREATE INDEX idx_template_tasks_template ON template_tasks(template_id);
CREATE INDEX idx_template_tasks_order ON template_tasks(template_id, "order");
