ALTER TABLE IF EXISTS "commit"   
    DROP CONSTRAINT "commit_environment_id_fkey",   
    ADD CONSTRAINT "commit_environment_id_fkey" FOREIGN KEY ("environment_id")
          REFERENCES "environment" ("id");
        


ALTER TABLE IF EXISTS "commit"   
    DROP CONSTRAINT "commit_project_id_fkey",   
    ADD CONSTRAINT "commit_project_id_fkey" FOREIGN KEY ("project_id")
          REFERENCES "project" ("id");

DROP TABLE IF EXISTS "commit";