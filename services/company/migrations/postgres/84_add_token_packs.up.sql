-- Token packs: add-on consumable AI-token balance that a project falls back to
-- once its fare's daily/monthly token limit is reached.

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'token_pack';
    END IF;
END;
$$;

-- Catalog of purchasable token packs (admin-managed, dynamic).
CREATE TABLE IF NOT EXISTS token_pack (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    token_amount    BIGINT NOT NULL CHECK (token_amount > 0),
    price           DECIMAL(20,2) NOT NULL DEFAULT 0,
    currency_id     UUID REFERENCES currency(id) ON DELETE SET NULL,
    product_type    product_type NOT NULL DEFAULT 'ugen'::product_type,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at      INTEGER NOT NULL DEFAULT 0
);

-- One consumable pack-token pool per company.
CREATE TABLE IF NOT EXISTS company_token_balance (
    company_id       UUID PRIMARY KEY,
    remaining_tokens BIGINT NOT NULL DEFAULT 0 CHECK (remaining_tokens >= 0),
    updated_at       TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Audit ledger of pack purchases; records which project paid for the pool top-up.
CREATE TABLE IF NOT EXISTS token_pack_purchase (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL,
    project_id      UUID NOT NULL,
    pack_id         UUID REFERENCES token_pack(id) ON DELETE SET NULL,
    token_amount    BIGINT NOT NULL,
    price           DECIMAL(20,2) NOT NULL DEFAULT 0,
    currency_id     UUID REFERENCES currency(id) ON DELETE SET NULL,
    transaction_id  UUID REFERENCES transaction(id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_token_pack_purchase_company ON token_pack_purchase (company_id, created_at);
CREATE INDEX IF NOT EXISTS idx_token_pack_purchase_project ON token_pack_purchase (project_id, created_at);

-- Portion of a usage row funded by the pack (excluded from plan day/month metrics
-- so the fare budget recovers cleanly each period).
ALTER TABLE ai_token_usage
    ADD COLUMN IF NOT EXISTS pack_tokens INT NOT NULL DEFAULT 0;
