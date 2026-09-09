CREATE TABLE IF NOT EXISTS "apple_settings" (
    "id" UUID PRIMARY KEY,
    "project_id" VARCHAR NOT Null,
    "team_id" VARCHAR NOT NULL,
    "client_id" VARCHAR NOT NULL,
    "key_id" VARCHAR NOT NULL,
    "secret" VARCHAR NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);