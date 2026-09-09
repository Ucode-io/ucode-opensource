-- transaction_type enum values ('user_seat_purchase', 'user_seat_refund') cannot
-- be removed in PostgreSQL and are intentionally left in place.
ALTER TABLE project
    DROP COLUMN IF EXISTS per_user_currency_id,
    DROP COLUMN IF EXISTS per_user_price;

ALTER TABLE ugen_template
    DROP COLUMN IF EXISTS per_user_price;
