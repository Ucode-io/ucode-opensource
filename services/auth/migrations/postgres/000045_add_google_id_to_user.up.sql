ALTER TABLE "user" ADD COLUMN IF NOT EXISTS google_id text;

CREATE UNIQUE INDEX IF NOT EXISTS user_unq_google_id
    ON "user" (google_id)
    WHERE NOT (google_id IS NULL OR google_id = ''::text);
