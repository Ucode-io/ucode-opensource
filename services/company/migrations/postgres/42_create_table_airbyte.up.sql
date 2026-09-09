DROP TABLE IF EXISTS airbyte_credentials;

CREATE TABLE IF NOT EXISTS airbyte (
    "id" UUID PRIMARY KEY,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "resource_id" UUID REFERENCES "resource"("id"),
    "schedule_time" integer,
    "workspace_id" VARCHAR NOT NULL,
    "source_id" VARCHAR NOT NULL,
    "destination_id" VARCHAR NOT NULL,
    "connection_id" VARCHAR NOT NULL,
    "connection_name" VARCHAR NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
