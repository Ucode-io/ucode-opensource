ALTER TABLE IF EXISTS "project_card"
    DROP COLUMN IF EXISTS "type",
    DROP COLUMN IF EXISTS "external_id";

ALTER tABLE IF EXISTS "project"
    DROP COLUMN IF EXISTS "customer_id";

ALTER TABLE IF EXISTS "transaction"
    DROP COLUMN IF EXISTS "external_id";
    

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