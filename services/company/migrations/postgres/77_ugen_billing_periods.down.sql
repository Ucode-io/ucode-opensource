ALTER TABLE subscription
    DROP COLUMN IF EXISTS canceled_at,
    DROP COLUMN IF EXISTS cancel_at_period_end,
    DROP COLUMN IF EXISTS billing_period_discount_percent,
    DROP COLUMN IF EXISTS billing_period_months,
    DROP COLUMN IF EXISTS billing_period_code;

DROP TABLE IF EXISTS ugen_billing_period;
