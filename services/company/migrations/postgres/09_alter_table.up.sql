ALTER TABLE IF EXISTS "resource_environment"   
    DROP CONSTRAINT "resource_environment_resource_id_fkey",   
    ADD CONSTRAINT "resource_environment_resource_id_fkey" FOREIGN KEY ("resource_id")
          REFERENCES "resource" ("id") ON DELETE CASCADE;