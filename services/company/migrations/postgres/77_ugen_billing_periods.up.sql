-- The hosted product's price list used to be seeded here. The open-source
-- build has no billing: the service code that read these tables was removed,
-- so the tables ship empty and the rows would only be commercial data with
-- nothing to read them. The schema itself stays, so a fork that wants billing
-- has somewhere to put it.

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

-- The billing periods and their discounts were seeded here; see the note
-- above. The table stays so the column below still has a target.

ALTER TABLE subscription
    ADD COLUMN IF NOT EXISTS billing_period_code VARCHAR(32) REFERENCES ugen_billing_period(code),
    ADD COLUMN IF NOT EXISTS billing_period_months INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS billing_period_discount_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS canceled_at TIMESTAMP WITHOUT TIME ZONE;
