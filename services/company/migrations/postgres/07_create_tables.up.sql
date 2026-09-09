CREATE TYPE access_type AS ENUM ('public', 'private');

CREATE TABLE IF NOT EXISTS "environment" (
    "id" UUID PRIMARY KEY,
    "project_id" UUID REFERENCES "project"("id"),
    "name" VARCHAR(255) NOT NULL,
    "display_color" VARCHAR(255) NOT NULL,
    "description" VARCHAR NOT NULL DEFAULT '',
    "access_type" access_type NOT NULL DEFAULT 'public',
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
 );