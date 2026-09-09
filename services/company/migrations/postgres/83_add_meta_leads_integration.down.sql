DROP INDEX IF EXISTS uq_integration_resource_project_env_type;
DROP INDEX IF EXISTS idx_project_resource_external_id;
DROP INDEX IF EXISTS uq_project_resource_external_id;

ALTER TABLE IF EXISTS "project_resource"
    DROP COLUMN IF EXISTS "external_id";

-- PostgreSQL enum values cannot be removed safely without recreating dependent columns.
