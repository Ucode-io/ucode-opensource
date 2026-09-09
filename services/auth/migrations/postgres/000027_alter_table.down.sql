-- PASSCODE
ALTER TABLE "passcode" DROP COLUMN "type";
ALTER TABLE "passcode" ADD COLUMN "phone" VARCHAR NOT NULL DEFAULT '';
ALTER TABLE "passcode" DROP COLUMN "item";

-- EMAIL SETTINGS
ALTER TABLE "email_settings" DROP COLUMN "env_id";
ALTER TABLE "email_settings" ADD CONSTRAINT "email_settings_project_id_key" UNIQUE(project_id);

-- LOGIN STRATEGY
-- DROP TYPE "login_strategy_type";
ALTER TABLE "login_strategy" DROP COLUMN "type";
ALTER TABLE "login_strategy" DROP COLUMN "project_id";
ALTER TABLE "login_strategy" DROP COLUMN "env_id";
ALTER TABLE "login_strategy" ADD COLUMN "view_field" VARCHAR(255);
ALTER TABLE "login_strategy" ADD COLUMN "name" VARCHAR(255);
ALTER TABLE "login_strategy" DROP COLUMN "created_at";

-- USER | INFO
ALTER TABLE "user_info" DROP COLUMN "project_id";
ALTER TABLE "user_info" DROP COLUMN "env_id";

-- SESSION
ALTER TABLE "session" DROP COLUMN "env_id";