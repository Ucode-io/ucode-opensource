ALTER TABLE IF EXISTS transactions 
    DROP COLUMN IF EXISTS subscription_id;
DROP TABLE IF EXISTS discount;
DROP TABLE IF EXISTS subscription;