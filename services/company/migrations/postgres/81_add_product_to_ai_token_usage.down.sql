DROP INDEX IF EXISTS idx_ai_token_usage_product;

ALTER TABLE ai_token_usage DROP COLUMN IF EXISTS product;