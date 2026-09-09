ALTER TABLE IF EXISTS "project_card"
    ADD COLUMN IF NOT EXISTS "type" VARCHAR DEFAULT 'UZCARD',
    ADD COLUMN IF NOT EXISTS "external_id" VARCHAR;

ALTER tABLE IF EXISTS "project"
    ADD COLUMN IF NOT EXISTS "customer_id" VARCHAR NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS "transaction"
    ADD COLUMN IF NOT EXISTS "external_id" VARCHAR NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "card_id" UUID REFERENCES "project_card"("id") ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS "rate" DECIMAL(20,2) NOT NULL DEFAULT 1.00;
    

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'payment_status'
    ) THEN
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requires_action';
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'processing';
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requires_payment_method';
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requires_confirmation';
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requires_capture';
        ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'succeeded';

    END IF;
END;
$$;