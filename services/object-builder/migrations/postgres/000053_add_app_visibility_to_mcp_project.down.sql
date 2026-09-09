ALTER TABLE "mcp_project"
DROP
COLUMN IF EXISTS "app_visibility";

DROP TYPE IF EXISTS app_visibility_enum;