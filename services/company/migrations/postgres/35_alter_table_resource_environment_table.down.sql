ALTER TABLE "resource_environment"
    DROP COLUMN IF EXISTS "node_type";
    
DROP TYPE IF EXISTS node_type_enum CASCADE;