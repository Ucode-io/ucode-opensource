DO
$$
BEGIN
    IF
NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'app_visibility_enum'
    ) THEN
CREATE TYPE app_visibility_enum AS ENUM ('public', 'private');
END IF;
END$$;

ALTER TABLE "mcp_project"
    ADD COLUMN IF NOT EXISTS "app_visibility" app_visibility_enum DEFAULT 'public';