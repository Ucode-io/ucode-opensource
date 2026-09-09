ALTER TABLE "user_project" ADD COLUMN IF NOT EXISTS "env_id" UUID;


DROP INDEX "user_project_idx_unique";
CREATE UNIQUE INDEX IF NOT EXISTS user_project_idx_unique
    ON "user_project" (user_id, company_id, project_id, client_type_id, role_id, env_id)
    WHERE client_type_id IS NOT NULL OR role_id IS NOT NULL OR env_id IS NOT NULL;