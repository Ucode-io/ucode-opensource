CREATE TABLE IF NOT EXISTS "commit" (
    "id" UUID PRIMARY KEY,
    "commit_id" INTEGER NOT NULL,
    "release_id" UUID,
    "project_id" UUID REFERENCES "project"("id"),
    "environment_id" UUID REFERENCES "environment"("id"),
    "author_id" UUID NOT NULL,
    "name" VARCHAR NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE IF EXISTS "commit"   
    DROP CONSTRAINT "commit_project_id_fkey",   
    ADD CONSTRAINT "commit_project_id_fkey" FOREIGN KEY ("project_id")
          REFERENCES "project" ("id") ON DELETE CASCADE;

ALTER TABLE IF EXISTS "commit"   
    DROP CONSTRAINT "commit_environment_id_fkey",   
    ADD CONSTRAINT "commit_environment_id_fkey" FOREIGN KEY ("environment_id")
          REFERENCES "environment" ("id") ON DELETE CASCADE;