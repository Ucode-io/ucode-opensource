DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'login_strategy_type'
    ) THEN
        ALTER TYPE login_strategy_type ADD VALUE IF NOT EXISTS 'GOOGLE_AUTH';
        ALTER TYPE login_strategy_type ADD VALUE IF NOT EXISTS 'APPLE_AUTH';
    END IF;
END;
$$;