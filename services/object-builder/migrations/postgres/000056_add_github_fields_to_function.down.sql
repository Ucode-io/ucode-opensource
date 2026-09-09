DROP INDEX IF EXISTS idx_function_github_repo_name;

ALTER TABLE function
    DROP COLUMN IF EXISTS github_repo_name,
    DROP COLUMN IF EXISTS github_webhook_id;
