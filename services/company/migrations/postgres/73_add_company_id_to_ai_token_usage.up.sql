ALTER TABLE ai_token_usage ADD COLUMN IF NOT EXISTS company_id UUID REFERENCES company(id);

CREATE INDEX IF NOT EXISTS idx_ai_token_usage_company_id ON ai_token_usage (company_id);
