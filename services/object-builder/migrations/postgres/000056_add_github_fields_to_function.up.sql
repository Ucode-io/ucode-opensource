ALTER TABLE function
    ADD COLUMN IF NOT EXISTS github_repo_name  VARCHAR NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS github_webhook_id VARCHAR NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_function_github_repo_name
    ON function (github_repo_name)
    WHERE github_repo_name <> '';
