-- Per-user pricing on a project: when set (> 0), adding a user charges the
-- price rather than enforcing a plan's seat limit. The template side of this
-- went with the Ugen surface; the project columns stay because the service
-- still reads and writes them.
ALTER TABLE project
    ADD COLUMN IF NOT EXISTS per_user_price       DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS per_user_currency_id UUID REFERENCES currency(id) ON DELETE SET NULL;

-- Transaction types for a paid user seat and its compensating refund.
DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'user_seat_purchase';
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'user_seat_refund';
    END IF;
END;
$$;
