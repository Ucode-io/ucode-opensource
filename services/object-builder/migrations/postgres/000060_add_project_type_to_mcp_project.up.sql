DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'project_type_enum') THEN
        CREATE TYPE project_type_enum AS ENUM ('admin_panel', 'web', 'landing', 'webapp');
    END IF;
END $$;

ALTER TABLE mcp_project
    ADD COLUMN IF NOT EXISTS project_type project_type_enum DEFAULT NULL;
