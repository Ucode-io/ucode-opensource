ALTER TABLE subscription DROP COLUMN IF EXISTS type;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'subscription_type') THEN
        DROP TYPE subscription_type;
    END IF;
END $$;
