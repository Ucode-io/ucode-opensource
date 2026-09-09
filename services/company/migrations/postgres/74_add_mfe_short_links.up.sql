CREATE TABLE IF NOT EXISTS mfe_short_links (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           VARCHAR(160) NOT NULL UNIQUE,
    url            TEXT         NOT NULL UNIQUE,
    project_id     UUID         REFERENCES project (id) ON DELETE SET NULL,
    mcp_project_id UUID         NOT NULL,
    function_id    UUID         NOT NULL,
    created_at     TIMESTAMP    NOT NULL DEFAULT NOW()
);
