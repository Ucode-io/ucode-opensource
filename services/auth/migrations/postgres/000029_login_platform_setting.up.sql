
-- CREATE TABLE IF NOT EXISTS email_settings (
--     "id" UUID PRIMARY KEY,
--     "project_id" UUID NOT NULL UNIQUE,
--     "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
--     "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
    
--     "email" VARCHAR NOT NULL,
--     "password" VARCHAR NOT NULL,
-- );

-- CREATE TABLE IF NOT EXISTS "apple_settings" (
--     "id" UUID PRIMARY KEY,
--     "project_id" VARCHAR NOT Null,
--     "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
--     "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
    
--     "team_id" VARCHAR NOT NULL,
--     "client_id" VARCHAR NOT NULL,
--     "key_id" VARCHAR NOT NULL,
--     "secret" VARCHAR NOT NULL,
-- );

-- //////////////////////////////////////////////////////////////////
CREATE TYPE "login_platform_type" AS ENUM (
  'APPLE',
  'GOOGLE'
);

CREATE TABLE "login_platform_setting" (
  "id" uuid PRIMARY KEY,
  "project_id" uuid,
  "env_id" uuid,
  "type" login_platform_type,
  "data" jsonB,
  "created_at" timestamp,
  "updated_at" timestamp
);