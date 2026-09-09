DROP INDEX IF EXISTS api_key_usage_api_key_idx;

ALTER TABLE IF EXISTS api_key_usage
    ADD COLUMN IF NOT EXISTS creation_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP(0);

ALTER TABLE IF EXISTS api_key_usage
    ADD COLUMN IF NOT EXISTS creation_month DATE DEFAULT TO_CHAR(DATE_TRUNC('month', CURRENT_TIMESTAMP), 'YYYY-MM-DD')::DATE;

CREATE INDEX IF NOT EXISTS api_key_usage_api_key_creation_time_idx 
    ON api_key_usage (api_key, creation_time);

CREATE INDEX IF NOT EXISTS api_key_usage_api_key_creation_month_idx 
    ON api_key_usage (api_key, creation_month);


ALTER TABLE IF EXISTS api_keys
    ADD COLUMN IF NOT EXISTS is_monthly_request_limit_reached BOOLEAN  DEFAULT FALSE;
-- MRL - monthlt request limit

ALTER TABLE IF EXISTS api_key_usage
    DROP COLUMN IF EXISTS request_count;


CREATE OR REPLACE FUNCTION update_monthly_limit_reached()
RETURNS TRIGGER AS $$
BEGIN
    IF (
        SELECT COUNT(*)
        FROM api_key_usage
        WHERE api_key = NEW.api_key
        AND creation_month = TO_CHAR(DATE_TRUNC('month', CURRENT_TIMESTAMP), 'YYYY-MM-DD')::DATE
    ) >= (
        SELECT monthly_request_limit
        FROM api_keys
        WHERE app_id = NEW.api_key
    ) THEN
        UPDATE api_keys
        SET is_monthly_request_limit_reached = TRUE
        WHERE app_id = NEW.api_key;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER check_monthly_limit_reached
BEFORE INSERT ON api_key_usage
FOR EACH ROW
EXECUTE FUNCTION update_monthly_limit_reached();
