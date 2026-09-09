CREATE TYPE resource_type AS ENUM (
    'MONGODB', 
    'POSTGRESQL',
    'CLICKHOUSE'
);

CREATE TYPE microservice_type AS ENUM (
    'node-object-builder-service', 
    'go-analytics-service'
);

CREATE TABLE IF NOT EXISTS "service_resource" (
    "id" UUID PRIMARY KEY,
    "resource_name" resource_type NOT NULL,
    "microservice_name" microservice_type NOT NULL,
    "name" VARCHAR NOT NULL,
    "description" VARCHAR DEFAULT '',
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);