CREATE TABLE IF NOT EXISTS ugen_template (
    id                     VARCHAR PRIMARY KEY,
    name                   VARCHAR NOT NULL,
    description            VARCHAR,
    photo                  VARCHAR,
    mcp_project_id         VARCHAR,
    preview_url            VARCHAR,
    source_resource_env_id VARCHAR,
    source_project_id      VARCHAR,
    source_environment_id  VARCHAR,
    source_node_type       VARCHAR,
    order_number           INT DEFAULT 0,
    created_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at             BIGINT DEFAULT 0
);

ALTER TABLE ugen_template
    ADD COLUMN IF NOT EXISTS source_mcp_resource_env_id VARCHAR,
    ADD COLUMN IF NOT EXISTS source_function_id VARCHAR,
    ADD COLUMN IF NOT EXISTS source_repo_id VARCHAR;
