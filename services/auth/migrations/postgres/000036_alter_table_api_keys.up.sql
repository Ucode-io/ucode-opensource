ALTER TABLE IF EXISTS api_keys 
    ADD COLUMN IF NOT EXISTS rps_limit INTEGER DEFAULT 2147483647;
ALTER TABLE IF EXISTS api_keys 
    ADD COLUMN IF NOT EXISTS monthly_request_limit INTEGER DEFAULT 2147483647;

CREATE TABLE IF NOT EXISTS api_key_usage (
    api_key VARCHAR not null,
    request_count integer not null default 0,
    created_at timestamp default now()
);