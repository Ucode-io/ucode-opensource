ALTER TABLE "user" ADD COLUMN tin text;

CREATE UNIQUE INDEX "user_unq_tin" ON "user" (tin) WHERE NOT (tin IS NULL OR tin = ''::text);

UPDATE "user" SET tin = '' WHERE tin IS NULL;