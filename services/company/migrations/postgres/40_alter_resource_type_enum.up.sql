DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'resource_type'
    ) THEN
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'GITHUB';
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'GITLAB';
        ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'BITBUCKET';

    END IF;
END;
$$;