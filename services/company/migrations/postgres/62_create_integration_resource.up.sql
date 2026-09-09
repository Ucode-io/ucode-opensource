CREATE TABLE IF NOT EXISTS integration_resource (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token          TEXT NOT NULL,
    project_id     UUID NOT NULL,
    environment_id UUID NOT NULL,
    username       TEXT NOT NULL DEFAULT '',
    name           TEXT NOT NULL DEFAULT '',
    type           resource_type NOT NULL,
    created_at     TIMESTAMP DEFAULT now(),
    updated_at     TIMESTAMP DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_integration_resource_project_env_type
    ON integration_resource (project_id, environment_id, type);