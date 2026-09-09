DROP TRIGGER IF EXISTS check_monthly_limit_reached ON api_key_usage;

DROP FUNCTION IF EXISTS update_monthly_limit_reached();

ALTER TABLE IF EXISTS api_keys
    DROP COLUMN IF EXISTS is_monthly_request_limit_reached;
-- MRL - monthlt request limit

DROP INDEX IF EXISTS api_key_usage_api_key_creation_time_idx;

DROP INDEX IF EXISTS api_key_usage_api_key_creation_month_idx;

ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS creation_time;

ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS creation_month;