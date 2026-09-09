ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'TELEGRAM';

ALTER TABLE IF EXISTS project_resource
    ADD COLUMN IF NOT EXISTS secret JSONB DEFAULT '{}'::JSONB;

-- A Telegram bot can have only one webhook. The unique index protects
-- generated projects from silently taking over each other's bot.
--
-- Do not use `type = 'TELEGRAM'` in this predicate. PostgreSQL rejects using a
-- newly-added enum value inside the same transaction where it was introduced.
CREATE UNIQUE INDEX IF NOT EXISTS project_resource_telegram_bot_id_unique
    ON project_resource ((settings #>> '{telegram,bot_id}'))
    WHERE settings ? 'telegram'
      AND COALESCE(settings #>> '{telegram,bot_id}', '') <> '';
