CREATE TABLE IF NOT EXISTS "release" (
    "id" UUID PRIMARY KEY,
    "commit_id" UUID REFERENCES "commit"("id"),
    "is_current" BOOLEAN NOT NULL,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "author_id" UUID NOT NULL,
    "version" VARCHAR(25) NOT NULL, -- e.g v0.1.4
    "description" VARCHAR DEFAULT '',
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE IF EXISTS "release"   
    DROP CONSTRAINT "release_project_id_fkey",   
    ADD CONSTRAINT "release_project_id_fkey" FOREIGN KEY ("project_id")
          REFERENCES "project" ("id") ON DELETE CASCADE;

ALTER TABLE IF EXISTS "release"   
    DROP CONSTRAINT "release_environment_id_fkey",   
    ADD CONSTRAINT "release_environment_id_fkey" FOREIGN KEY ("environment_id")
          REFERENCES "environment" ("id") ON DELETE CASCADE;

