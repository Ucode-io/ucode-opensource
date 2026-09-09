ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "salary" varchar(255);

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "work_place" varchar(255);

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "passport_number" varchar(7);

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "passport_serial" varchar(2);

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "expires_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL;

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "active" smallint;