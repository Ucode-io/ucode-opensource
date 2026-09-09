ALTER TABLE transaction DROP COLUMN IF EXISTS payment_type;

DROP TYPE IF EXISTS payment_type;
