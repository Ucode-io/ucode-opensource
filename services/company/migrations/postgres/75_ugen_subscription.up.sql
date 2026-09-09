ALTER TABLE subscription
    DROP CONSTRAINT IF EXISTS subscription_status_check;

ALTER TABLE subscription
    ADD CONSTRAINT subscription_status_check
    CHECK (status IN ('active', 'canceled', 'paused', 'expired', 'pending_downgrade'));

ALTER TABLE subscription
    ADD COLUMN IF NOT EXISTS pending_fare_id UUID REFERENCES fare(id) ON DELETE SET NULL;

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'upgrade';
    END IF;
END;
$$;
