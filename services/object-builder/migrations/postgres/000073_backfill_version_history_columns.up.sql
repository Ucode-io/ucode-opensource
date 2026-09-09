-- Heal schema drift on tenant `version_history` tables.
--
-- Migrations 000051/000052/000055 added method_api, time_started, time_completed,
-- duration, status_code and table_label. On tenants whose DB was cloned/moved with a
-- schema_migrations row already at the latest version but an older version_history
-- shape (e.g. template clones, the pscloud DB move), golang-migrate reports
-- ErrNoChange and never re-applies 51/52/55 -> the columns stay missing and the
-- version-history listing query fails with `column "..." does not exist` (42703).
--
-- This migration has a version higher than any tenant currently holds, so m.Up()
-- WILL run it on every tenant on the next reconnect. Every statement is
-- ADD COLUMN IF NOT EXISTS, so it is a no-op on healthy tenants and a repair on
-- drifted ones. ALTER on the parent propagates to partitions if a tenant's table
-- was manually partitioned.
ALTER TABLE "version_history"
    ADD COLUMN IF NOT EXISTS "method_api"     character varying(255),
    ADD COLUMN IF NOT EXISTS "time_started"   character varying(255),
    ADD COLUMN IF NOT EXISTS "time_completed" character varying(255),
    ADD COLUMN IF NOT EXISTS "duration"       bigint,
    ADD COLUMN IF NOT EXISTS "status_code"    bigint,
    ADD COLUMN IF NOT EXISTS "table_label"    character varying(255) DEFAULT '';
