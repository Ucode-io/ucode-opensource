ALTER TABLE ai_token_usage ADD COLUMN IF NOT EXISTS product product_type NOT NULL DEFAULT 'ugen'::product_type;

CREATE INDEX IF NOT EXISTS idx_ai_token_usage_product ON ai_token_usage (product);