ALTER TABLE mcp_project
    ADD COLUMN IF NOT EXISTS microfrontend_id UUID DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS microfrontend_repo_id TEXT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS microfrontend_branch TEXT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS microfrontend_url TEXT DEFAULT NULL;

ALTER TABLE "function"
    ADD COLUMN IF NOT EXISTS mcp_project_id UUID DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS mcp_resource_env_id UUID DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_function_repo_id
    ON "function" (repo_id)
    WHERE repo_id IS NOT NULL AND repo_id <> '';
