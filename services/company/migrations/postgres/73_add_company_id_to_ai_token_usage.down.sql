DROP INDEX IF EXISTS idx_ai_token_usage_company_id;

ALTER TABLE ai_token_usage DROP COLUMN IF EXISTS company_id;
