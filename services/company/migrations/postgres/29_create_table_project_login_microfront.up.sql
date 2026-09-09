CREATE TABLE IF NOT EXISTS project_login_microfront (
    "id" UUID PRIMARY KEY,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "microfront_id" VARCHAR,
    "subdomain" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);