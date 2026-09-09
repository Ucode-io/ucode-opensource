ALTER TABLE IF EXISTS "session" ADD COLUMN IF NOT EXISTS "user_id_auth" UUID;
UPDATE "session" SET "user_id_auth" = "user_id";