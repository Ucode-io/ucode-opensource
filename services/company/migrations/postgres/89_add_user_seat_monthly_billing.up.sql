-- Monthly recurring billing for users invited into projects created from paid
-- Ugen templates. The immediate invite charge remains user_seat_purchase; this
-- table records one recurring charge attempt per generated project per month.
DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'user_seat_monthly';
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'user_seat_monthly_refund';
    END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS project_user_seat_billing_period (
    id                  UUID PRIMARY KEY,
    project_id          UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    head_project_id     UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    period_month        DATE NOT NULL,
    total_user_count    INTEGER NOT NULL DEFAULT 0,
    billable_user_count INTEGER NOT NULL DEFAULT 0,
    per_user_price      DECIMAL(20,2) NOT NULL DEFAULT 0,
    currency_id         UUID REFERENCES currency(id) ON DELETE SET NULL,
    amount              DECIMAL(20,2) NOT NULL DEFAULT 0,
    charged_amount      DECIMAL(20,2) NOT NULL DEFAULT 0,
    transaction_id      UUID REFERENCES transaction(id) ON DELETE SET NULL,
    external_id         VARCHAR NOT NULL DEFAULT '',
    status              VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'insufficient_funds', 'failed', 'no_billable_seats')),
    last_error          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (project_id, period_month),
    UNIQUE (external_id)
);

CREATE INDEX IF NOT EXISTS idx_project_user_seat_billing_period_status
    ON project_user_seat_billing_period (status, period_month);
