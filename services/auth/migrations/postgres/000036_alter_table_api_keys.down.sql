ALTER TABLE IF EXISTS api_keys 
    DROP COLUMN IF EXISTS rps_limit;
ALTER TABLE IF EXISTS api_keys 
    DROP COLUMN IF EXISTS monthly_request_limit;

DROP TABLE IF EXISTS api_key_usage;