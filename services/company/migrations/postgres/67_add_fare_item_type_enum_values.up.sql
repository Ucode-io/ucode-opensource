DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'fare_item_type') THEN
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'asset_size';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'users_count';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'items';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'tables';
        ALTER TYPE fare_item_type ADD VALUE IF NOT EXISTS 'api_keys';
    END IF;
END;
$$;