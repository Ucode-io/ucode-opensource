ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'META_LEADS';

ALTER TABLE IF EXISTS "project_resource"
    ADD COLUMN IF NOT EXISTS "external_id" VARCHAR NOT NULL DEFAULT '';

-- Resolve an incoming webhook to its project_resource row(s) by secondary key
-- (e.g. Meta page_id). Not globally unique: one page may be wired to several
-- projects/environments, so a lead fans out to each.
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_resource_external_id
    ON "project_resource" ("external_id", "project_id", "environment_id")
    WHERE "external_id" <> '';

CREATE INDEX IF NOT EXISTS idx_project_resource_external_id
    ON "project_resource" ("external_id")
    WHERE "external_id" <> '';

-- One integration connection (e.g. a Meta user token) per project environment and
-- integration type, so the OAuth callback can upsert it atomically via ON CONFLICT.
CREATE UNIQUE INDEX IF NOT EXISTS uq_integration_resource_project_env_type
    ON "integration_resource" ("project_id", "environment_id", "type");
