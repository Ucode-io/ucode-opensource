ALTER TABLE ai_token_usage DROP COLUMN IF EXISTS pack_tokens;

DROP INDEX IF EXISTS idx_token_pack_purchase_project;
DROP INDEX IF EXISTS idx_token_pack_purchase_company;

DROP TABLE IF EXISTS token_pack_purchase;
DROP TABLE IF EXISTS company_token_balance;
DROP TABLE IF EXISTS token_pack;

-- Note: the 'token_pack' value on enum transaction_type is intentionally left in
-- place; PostgreSQL does not support removing a value from an enum type.
