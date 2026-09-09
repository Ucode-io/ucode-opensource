ALTER TABLE "user_project" DROP COLUMN IF EXISTS "env_id";

DROP INDEX IF EXISTS "user_project_idx_unique";
CREATE UNIQUE INDEX IF NOT EXISTS user_project_idx_unique
    ON "user_project" (user_id, company_id, project_id, client_type_id, role_id)
    WHERE client_type_id IS NOT NULL OR role_id IS NOT NULL;