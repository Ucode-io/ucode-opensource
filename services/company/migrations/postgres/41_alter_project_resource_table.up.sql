DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'resource_type'
    ) THEN
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'SMS';
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'SMTP';
    END IF;
END;
$$;

ALTER TABLE IF EXISTS project_resource
    ADD COLUMN IF NOT EXISTS settings JSONB DEFAULT '{}'::JSONB;