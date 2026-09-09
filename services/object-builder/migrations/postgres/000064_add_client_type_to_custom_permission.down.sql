DROP INDEX IF EXISTS "custom_permission_client_type_id_idx";

ALTER TABLE IF EXISTS "custom_permission"
    DROP COLUMN IF EXISTS "client_type_id";
