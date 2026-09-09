ALTER TABLE mcp_project
    DROP COLUMN IF EXISTS project_type;

DROP TYPE IF EXISTS project_type_enum;
