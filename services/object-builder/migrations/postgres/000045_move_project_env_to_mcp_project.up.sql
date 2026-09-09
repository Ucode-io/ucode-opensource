ALTER TABLE "project_files"
DROP COLUMN IF EXISTS project_env;

ALTER TABLE "mcp_project"
    ADD COLUMN IF NOT EXISTS project_env JSONB DEFAULT '{}';