ALTER TABLE IF EXISTS "api_keys"
    ADD COLUMN IF NOT EXISTS "client_type_id" UUID;

ALTER TABLE IF EXISTS "api_keys" RENAME COLUMN resource_environment_id TO environment_id;