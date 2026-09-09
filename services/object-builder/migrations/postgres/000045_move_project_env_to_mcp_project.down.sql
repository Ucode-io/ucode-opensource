ALTER TABLE "mcp_project" DROP COLUMN IF EXISTS "project_env";

ALTER TABLE "project_files" ADD COLUMN IF NOT EXISTS "project_env" JSONB DEFAULT '{}';
