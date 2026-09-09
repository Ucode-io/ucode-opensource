ALTER TABLE IF EXISTS api_keys
    ALTER COLUMN rps_limit DROP DEFAULT;

ALTER TABLE IF EXISTS api_keys
    ALTER COLUMN monthly_request_limit DROP DEFAULT;

DROP INDEX IF EXISTS api_key_usage_api_key_idx;