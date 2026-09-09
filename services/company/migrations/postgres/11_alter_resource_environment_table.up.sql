ALTER TABLE IF EXISTS "resource_environment"
    ADD COLUMN IF NOT EXISTS "default" boolean NOT NULL default false,
    ADD CONSTRAINT resouce_environment_unique UNIQUE (resource_id, environment_id)