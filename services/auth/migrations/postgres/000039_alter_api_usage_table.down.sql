ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS request_count;

ALTER TABLE IF EXISTS api_key_usage
    DROP CONSTRAINT api_key_usage_api_key_creation_month_u_idx;

ALTER TABLE IF EXISTS api_key_usage
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE IF EXISTS api_key_usage
    ADD COLUMN IF NOT EXISTS creation_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP(0);

CREATE INDEX IF NOT EXISTS api_key_usage_api_key_creation_time_idx 
    ON api_key_usage (api_key, creation_time);

CREATE INDEX IF NOT EXISTS api_key_usage_api_key_creation_month_idx 
    ON api_key_usage (api_key, creation_month);

