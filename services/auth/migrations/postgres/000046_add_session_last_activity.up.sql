ALTER TABLE IF EXISTS "session" ADD COLUMN IF NOT EXISTS "last_activity_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL;
UPDATE "session" SET "last_activity_at" = "updated_at";
