ALTER TABLE "version_history"
ADD COLUMN IF NOT EXISTS "status_code" bigint;
