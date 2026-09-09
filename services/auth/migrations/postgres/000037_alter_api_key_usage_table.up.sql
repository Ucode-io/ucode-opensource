ALTER TABLE IF EXISTS api_keys
    ALTER COLUMN rps_limit SET DEFAULT 10;

ALTER TABLE IF EXISTS api_keys
    ALTER COLUMN monthly_request_limit SET DEFAULT 2000000;

CREATE INDEX IF NOT EXISTS api_key_usage_api_key_idx ON api_key_usage (api_key);