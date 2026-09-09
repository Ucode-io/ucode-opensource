CREATE TABLE IF NOT EXISTS "integration_resource" (
    "id" UUID PRIMARY KEY,
    "token" VARCHAR,
    "name" VARCHAR,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "username" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("project_id", "environment_id", "username")
);
