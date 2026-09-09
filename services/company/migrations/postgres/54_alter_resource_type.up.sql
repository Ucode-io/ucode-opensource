DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'resource_type'
    ) THEN
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'METABASE';
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'MAILCHIMP';
    END IF;
END;
$$;