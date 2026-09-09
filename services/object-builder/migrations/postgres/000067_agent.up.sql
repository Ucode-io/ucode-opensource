CREATE TABLE IF NOT EXISTS agents
(
    id          UUID PRIMARY KEY      DEFAULT uuid_generate_v4(),
    project_id  UUID         NOT NULL REFERENCES mcp_project (id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    instruction TEXT         NOT NULL DEFAULT '',
    model       VARCHAR(100) NOT NULL DEFAULT 'claude-sonnet-4-5',
    max_steps   INT          NOT NULL DEFAULT 8,
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_permissions
(
    id         UUID PRIMARY KEY      DEFAULT uuid_generate_v4(),
    agent_id   UUID         NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    table_slug VARCHAR(255) NOT NULL,
    can_create BOOLEAN      NOT NULL DEFAULT FALSE,
    can_read   BOOLEAN      NOT NULL DEFAULT FALSE,
    can_update BOOLEAN      NOT NULL DEFAULT FALSE,
    can_delete BOOLEAN      NOT NULL DEFAULT FALSE,
    can_list   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (agent_id, table_slug)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'agent_run_status' AND typtype = 'e') THEN
        CREATE TYPE agent_run_status AS ENUM ('running', 'succeeded', 'failed');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS agent_runs
(
    id          UUID PRIMARY KEY          DEFAULT uuid_generate_v4(),
    agent_id    UUID             NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    project_id  UUID             NOT NULL REFERENCES mcp_project (id) ON DELETE CASCADE,
    status      agent_run_status NOT NULL DEFAULT 'running',
    input       JSONB            NOT NULL DEFAULT '{}',
    output      TEXT             NOT NULL DEFAULT '',
    steps       JSONB            NOT NULL DEFAULT '[]',
    tokens_used INT              NOT NULL DEFAULT 0,
    error       TEXT             NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_agent_id ON agent_runs (agent_id);
