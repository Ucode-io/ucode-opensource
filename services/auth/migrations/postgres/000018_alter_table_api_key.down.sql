ALTER TABLE "api_keys" DROP COLUMN IF EXISTS "client_type_id";

ALTER TABLE IF EXISTS "api_keys" RENAME COLUMN environment_id TO resource_environment_id;