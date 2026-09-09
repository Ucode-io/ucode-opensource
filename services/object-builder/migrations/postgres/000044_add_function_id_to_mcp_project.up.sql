ALTER TABLE "mcp_project"
    ADD COLUMN IF NOT EXISTS "function_id" UUID
        REFERENCES function (id)
        ON DELETE SET NULL
        DEFAULT NULL;