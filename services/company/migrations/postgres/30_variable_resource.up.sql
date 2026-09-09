CREATE TABLE IF NOT EXISTS variable_resource (
    "id" UUID PRIMARY KEY,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "key" VARCHAR,
    "value" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);