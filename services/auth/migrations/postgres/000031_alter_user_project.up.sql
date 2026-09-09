ALTER TABLE "user_project" ADD COLUMN IF NOT EXISTS "client_type_id" UUID;
ALTER TABLE "user_project" ADD COLUMN IF NOT EXISTS "role_id" UUID;