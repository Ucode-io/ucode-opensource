ALTER TABLE IF EXISTS "custom_permission"
    ADD COLUMN IF NOT EXISTS "client_type_id" UUID REFERENCES "client_type"("guid") ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS "custom_permission_client_type_id_idx"
    ON "custom_permission"("client_type_id");
