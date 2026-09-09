DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'subscription_type') THEN
        CREATE TYPE subscription_type AS ENUM ('free_trial', 'paid');
    END IF;
END $$;

ALTER TABLE subscription
ADD COLUMN type subscription_type NOT NULL DEFAULT 'paid';
