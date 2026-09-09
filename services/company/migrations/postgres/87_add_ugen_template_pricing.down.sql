-- transaction_type enum values ('template_purchase', 'template_refund') cannot
-- be removed in PostgreSQL and are intentionally left in place.
DROP INDEX IF EXISTS idx_transaction_external_id;

ALTER TABLE ugen_template
    DROP COLUMN IF EXISTS currency_id,
    DROP COLUMN IF EXISTS price;
