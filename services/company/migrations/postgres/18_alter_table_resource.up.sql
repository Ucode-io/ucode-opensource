CREATE TYPE resource_type AS ENUM (
    'NOT_DECIDED',
    'MONGODB',
    'POSTGRESQL',
    'CLICKHOUSE'
);

ALTER TABLE resource ADD COLUMN "resource_type" resource_type NOT NULL DEFAULT 'NOT_DECIDED';
ALTER TABLE resource ADD COLUMN "is_configured" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE resource ADD COLUMN "vault_path" VARCHAR NOT NULL DEFAULT '';