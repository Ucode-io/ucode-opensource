CREATE TYPE "node_type_enum" AS ENUM (
    'HIGH', 
    'LOW'
);

ALTER TABLE IF EXISTS "resource_environment"
    ADD COLUMN IF NOT EXISTS "node_type" node_type_enum NOT NULL DEFAULT 'LOW';