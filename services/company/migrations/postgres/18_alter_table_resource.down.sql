DROP TYPE IF EXISTS resource_type;

ALTER TABLE resource DROP COLUMN IF EXISTS "resource_type";
ALTER TABLE resource DROP COLUMN IF EXISTS "is_configured";
ALTER TABLE resource DROP COLUMN IF EXISTS "vault_path";