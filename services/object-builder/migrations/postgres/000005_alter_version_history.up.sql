DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'version_history'
        AND column_name = 'used_environments'
    ) THEN
        ALTER TABLE "version_history"
        ADD COLUMN "used_environments" JSONB DEFAULT '{}';
    END IF;
END $$;
