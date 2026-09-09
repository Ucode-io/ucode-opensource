CREATE TABLE IF NOT EXISTS project_resource (
    "id" UUID PRIMARY KEY,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "name" VARCHAR NOT NULL,
    "type" resource_type NOT NULL DEFAULT 'NOT_DECIDED',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);