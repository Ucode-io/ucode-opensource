CREATE UNIQUE INDEX IF NOT EXISTS "unq_env_project_key"
    ON "variable_resource" (environment_id, project_id, key);