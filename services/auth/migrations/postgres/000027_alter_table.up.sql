-- PASSCODE
CREATE TYPE passcode_type AS ENUM('PHONE', 'EMAIL');

ALTER TABLE "passcode" ADD COLUMN "type" passcode_type NOT NULL;
ALTER TABLE "passcode" DROP COLUMN "phone";
ALTER TABLE "passcode" ADD COLUMN "item" VARCHAR;

-- EMAIL SETTINGS
ALTER TABLE "email_settings" ADD COLUMN "env_id" UUID NOT NULL DEFAULT 'b826f23b-403f-4775-bfe3-4a1ccf4e006e'::UUID;
ALTER TABLE "email_settings" DROP CONSTRAINT "email_settings_project_id_key";
ALTER TABLE "email_settings" ADD CONSTRAINT "email_settings_project_id_env_id_key" UNIQUE(project_id, env_id);

-- LOGIN STRATEGY
CREATE TYPE "login_strategy_type" AS ENUM('PHONE', 'EMAIL', 'LOGIN', 'PHONE_OTP', 'EMAIL_OTP', 'LOGIN_PWD');
ALTER TABLE "login_strategy" ADD COLUMN "type" login_strategy_type NOT NULL;
ALTER TABLE "login_strategy" ADD COLUMN "project_id" UUID NOT NULL;
ALTER TABLE "login_strategy" ADD COLUMN "env_id" UUID NOT NULL;
ALTER TABLE "login_strategy" DROP COLUMN "view_field";
ALTER TABLE "login_strategy" DROP COLUMN "name";
ALTER TABLE "login_strategy" ADD COLUMN "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL;

-- USER | INFO
CREATE TABLE IF NOT EXISTS "user_info" (
    "user_id" UUID REFERENCES "user"("id"),
    "data" JSONB,
    "project_id" UUID NOT NULL,
    "env_id" UUID NOT NULL,
    UNIQUE(user_id, project_id, env_id)
);

-- SESSION
DELETE FROM "session";
ALTER TABLE "session" ADD COLUMN IF NOT EXISTS "env_id" UUID;