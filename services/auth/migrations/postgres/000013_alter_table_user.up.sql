ALTER TABLE "user" DROP COLUMN  IF EXISTS project_id;

ALTER TABLE "user" ADD COLUMN company_id uuid;