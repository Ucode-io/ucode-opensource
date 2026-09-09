CREATE TABLE IF NOT EXISTS ugen_billing_period (
    code                VARCHAR(32) PRIMARY KEY,
    name                VARCHAR(64) NOT NULL,
    months              INTEGER NOT NULL CHECK (months > 0),
    discount_percent    DECIMAL(5,2) NOT NULL DEFAULT 0 CHECK (discount_percent >= 0 AND discount_percent <= 100),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO ugen_billing_period (code, name, months, discount_percent, is_active, sort_order)
VALUES
    ('monthly', 'Monthly', 1, 0, TRUE, 1),
    ('six_months', '6 months', 6, 13, TRUE, 2),
    ('annual', 'Annual', 12, 24, TRUE, 3)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    months = EXCLUDED.months,
    discount_percent = EXCLUDED.discount_percent,
    is_active = EXCLUDED.is_active,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

ALTER TABLE subscription
    ADD COLUMN IF NOT EXISTS billing_period_code VARCHAR(32) REFERENCES ugen_billing_period(code) DEFAULT 'monthly',
    ADD COLUMN IF NOT EXISTS billing_period_months INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS billing_period_discount_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS canceled_at TIMESTAMP WITHOUT TIME ZONE;
