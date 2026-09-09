ALTER TABLE "session" DROP CONSTRAINT IF EXISTS "session_client_platform_id_fkey";
ALTER TABLE "session" DROP CONSTRAINT IF EXISTS "session_client_type_id_fkey";
ALTER TABLE "session" DROP CONSTRAINT IF EXISTS "session_user_id_fkey";
ALTER TABLE "session" DROP CONSTRAINT IF EXISTS "session_role_id_fkey";
ALTER TABLE "scope" DROP CONSTRAINT IF EXISTS "scope_client_platform_id_fkey";