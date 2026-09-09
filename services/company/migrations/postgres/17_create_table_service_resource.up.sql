DROP TABLE IF EXISTS migrations_ucode_go_versioning_service;
DROP TABLE IF EXISTS release;
DROP TABLE IF EXISTS commit;

DROP TABLE IF EXISTS service_resource;

DROP TYPE IF EXISTS "microservice_type";
DROP TYPE IF EXISTS "resource_type";

CREATE TYPE service_type AS ENUM (
    'BUILDER_SERVICE',
    'ANALYTICS_SERVICE',
    'TEMPLATE_SERVICE',
    'QUERY_SERVICE',
    'FUNCTION_SERVICE',
    'WEB_PAGE_SERVICE'
);

CREATE TABLE IF NOT EXISTS service_resource (
    "id" UUID PRIMARY KEY,
    "service_type" service_type NOT NULL,
    "project_id" UUID NOT NULL REFERENCES project("id"),
    "title" VARCHAR NOT NULL,
    "description" VARCHAR DEFAULT '',
    "resource_environment_id" UUID NOT NULL REFERENCES resource_environment("id"),
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE service_resource ADD CONSTRAINT unique_project_id_service_type UNIQUE(project_id, service_type);