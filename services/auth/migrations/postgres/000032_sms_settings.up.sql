CREATE TABLE IF NOT EXISTS sms_otp_settings (
    "id" UUID PRIMARY KEY,
    "login" VARCHAR NOT NULL,
    "password" VARCHAR NOT NULL,
    "project_id" UUID NOT NULL,
    "environment_id" UUID NOT NULL,
    "number_of_otp" INTEGER NOT NULL DEFAULT 4,
    "default_otp" VARCHAR DEFAULT '',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE (project_id, environment_id)
);