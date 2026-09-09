DROP TABLE IF EXISTS transaction CASCADE;
DROP TABLE IF EXISTS fare_item_price CASCADE;
DROP TABLE IF EXISTS fare_item CASCADE;
DROP TABLE IF EXISTS fare CASCADE;

-- Drop columns added to the "project" table
ALTER TABLE IF EXISTS project
    DROP COLUMN IF EXISTS balance,
    DROP COLUMN IF EXISTS credit_limit;
    DROP COLUMN IF EXISTS fare_id;

-- Drop ENUM types if they exist
DROP TYPE IF EXISTS payment_status CASCADE;
DROP TYPE IF EXISTS transaction_type CASCADE;
DROP TYPE IF EXISTS creator_type CASCADE;
