-- Ugen template pricing: a template may carry a price that is charged to the
-- head project's balance when a project is provisioned from it. Free templates
-- keep price = 0 and are never charged.
ALTER TABLE ugen_template
    ADD COLUMN IF NOT EXISTS price       DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS currency_id UUID REFERENCES currency(id) ON DELETE SET NULL;

-- Transaction types for a template charge and its compensating refund.
DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'template_purchase';
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'template_refund';
    END IF;
END;
$$;

-- Speeds up the idempotency lookup that guards a retried template charge from
-- double-debiting (transaction.external_id already exists from migration 58).
CREATE INDEX IF NOT EXISTS idx_transaction_external_id
    ON transaction (external_id)
    WHERE external_id <> '';