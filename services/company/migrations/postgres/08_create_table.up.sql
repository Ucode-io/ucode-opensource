CREATE TABLE IF NOT EXISTS "resource_environment" (
    "id" UUID PRIMARY KEY,
	"project_id" UUID REFERENCES "project"("id"),
    "resource_id" UUID REFERENCES "resource"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "databasename" VARCHAR NOT NULL,
	"is_configured" BOOLEAN NOT NULL,
    "updated_at" TIMESTAMP,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
 );