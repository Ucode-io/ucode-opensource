CREATE TABLE IF NOT EXISTS "resource" (
    "id" UUID PRIMARY KEY,
    "vault_path" VARCHAR NOT NULL,
    "project_id" UUID REFERENCES "project"("id"),
    "service_type" SMALLINT NOT NULL,
    "resource_type" SMALLINT NOT NULL,
    "host" VARCHAR(255) NOT NULL,
    "port" INTEGER NOT NULL,
    "username" VARCHAR(255) NOT NULL,
    "database" VARCHAR(255) NOT NULL,
    "updated_at" TIMESTAMP,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
 );