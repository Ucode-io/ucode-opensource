ALTER TABLE "resource_environment"
    DROP COLUMN IF EXISTS "default",
    DROP CONSTRAINT IF EXISTS resouce_environment_unique