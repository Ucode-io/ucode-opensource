DROP INDEX IF EXISTS idx_function_repo_id;

ALTER TABLE "function"
    DROP COLUMN IF EXISTS mcp_project_id,
    DROP COLUMN IF EXISTS mcp_resource_env_id;

ALTER TABLE mcp_project
    DROP COLUMN IF EXISTS microfrontend_id,
    DROP COLUMN IF EXISTS microfrontend_repo_id,
    DROP COLUMN IF EXISTS microfrontend_branch,
    DROP COLUMN IF EXISTS microfrontend_url;
