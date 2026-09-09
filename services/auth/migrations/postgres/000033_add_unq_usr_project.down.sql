ALTER TABLE "user_project" DROP CONSTRAINT IF EXISTS "user_project_idx_unique";
CREATE UNIQUE INDEX IF NOT EXISTS "user_project_user_id_company_id_project_id_key"
    ON "user_project" (user_id, company_id, project_id);