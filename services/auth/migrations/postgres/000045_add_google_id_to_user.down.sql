DROP INDEX IF EXISTS user_unq_google_id;

ALTER TABLE "user" DROP COLUMN IF EXISTS google_id;
