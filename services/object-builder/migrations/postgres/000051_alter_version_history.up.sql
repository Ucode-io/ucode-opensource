ALTER TABLE "version_history"
ADD COLUMN IF NOT EXISTS "method_api" character varying(255),
ADD COLUMN IF NOT EXISTS "time_started" character varying(255),
ADD COLUMN IF NOT EXISTS "time_completed" character varying(255),
ADD COLUMN IF NOT EXISTS "duration" bigint;
