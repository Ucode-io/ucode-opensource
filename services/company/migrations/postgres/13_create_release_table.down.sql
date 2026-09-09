ALTER TABLE IF EXISTS "release"   
    DROP CONSTRAINT "release_environment_id_fkey",   
    ADD CONSTRAINT "release_environment_id_fkey" FOREIGN KEY ("environment_id")
          REFERENCES "environment" ("id");
        


ALTER TABLE IF EXISTS "release"   
    DROP CONSTRAINT "release_project_id_fkey",   
    ADD CONSTRAINT "release_project_id_fkey" FOREIGN KEY ("project_id")
          REFERENCES "project" ("id");


DROP TABLE IF EXISTS "release";