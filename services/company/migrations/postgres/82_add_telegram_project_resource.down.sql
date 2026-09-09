DROP INDEX IF EXISTS project_resource_telegram_bot_id_unique;

ALTER TABLE IF EXISTS project_resource
    DROP COLUMN IF EXISTS secret;

-- PostgreSQL does not support safely removing enum values from an existing type.
