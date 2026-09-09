DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'fare_item_type') THEN
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'tokens_day';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'tokens_month';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'builders';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'projects';
    END IF;
END;
$$;