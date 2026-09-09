CREATE TABLE "mcp_project"
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title       VARCHAR(255)     DEFAULT '',
    description VARCHAR(255)     DEFAULT '',
    created_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP        DEFAULT NULL
);

CREATE TABLE "project_files"
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID REFERENCES mcp_project (id) NOT NULL,
    file_path   VARCHAR(255)                     NOT NULL,
    content     TEXT             DEFAULT '',
    file_graph  JSONB            DEFAULT '{}',
    project_env JSONB            DEFAULT '{}',
    created_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP        DEFAULT NULL,

    CONSTRAINT uq_project_file_path UNIQUE (project_id, file_path)
);