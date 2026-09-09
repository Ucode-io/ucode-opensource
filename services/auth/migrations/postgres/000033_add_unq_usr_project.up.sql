ALTER TABLE "user_project" DROP CONSTRAINT IF EXISTS "user_project_user_id_company_id_project_id_key";
CREATE UNIQUE INDEX IF NOT EXISTS user_project_idx_unique
    ON "user_project" (user_id, company_id, project_id, client_type_id, role_id)
    WHERE client_type_id IS NOT NULL OR role_id IS NOT NULL;