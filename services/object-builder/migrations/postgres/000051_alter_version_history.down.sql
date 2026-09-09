ALTER TABLE "version_history"
DROP COLUMN IF EXISTS "method_api",
DROP COLUMN IF EXISTS "time_started",
DROP COLUMN IF EXISTS "time_completed",
DROP COLUMN IF EXISTS "duration";
