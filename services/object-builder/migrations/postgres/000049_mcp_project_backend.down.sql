ALTER TABLE "mcp_project"
    DROP COLUMN IF EXISTS "ucode_project_id",
    DROP COLUMN IF EXISTS "api_key",
    DROP COLUMN IF EXISTS "environment_id",
    DROP COLUMN IF EXISTS "status";
