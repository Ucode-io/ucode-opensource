-- Per-user pricing for template-derived projects: a Ugen template may carry a
-- per_user_price that is copied onto every project provisioned from it. When set
-- (> 0), adding a user to that project charges the price to the company's head
-- project balance instead of enforcing the fare-based user limit. Ordinary
-- projects keep per_user_price = 0 and the existing limit behaviour.
ALTER TABLE ugen_template
    ADD COLUMN IF NOT EXISTS per_user_price DECIMAL(20,2) NOT NULL DEFAULT 0;

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
