ALTER TABLE subscription
    DROP COLUMN IF EXISTS pending_fare_id;

ALTER TABLE subscription
    DROP CONSTRAINT IF EXISTS subscription_status_check;

ALTER TABLE subscription
    ADD CONSTRAINT subscription_status_check
    CHECK (status IN ('active', 'canceled', 'paused', 'expired'));
