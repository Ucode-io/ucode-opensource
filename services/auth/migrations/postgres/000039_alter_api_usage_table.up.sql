DROP TRIGGER IF EXISTS check_monthly_limit_reached ON api_key_usage;

DROP FUNCTION IF EXISTS update_monthly_limit_reached();

DROP INDEX IF EXISTS api_key_usage_api_key_creation_time_idx;

DROP INDEX IF EXISTS api_key_usage_api_key_creation_month_idx;

ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS creation_time;

ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS created_at;

ALTER TABLE IF EXISTS api_key_usage
    ADD COLUMN IF NOT EXISTS request_count INTEGER DEFAULT 0;

ALTER TABLE api_key_usage
    ADD CONSTRAINT api_key_usage_api_key_creation_month_u_idx UNIQUE (api_key, creation_month);