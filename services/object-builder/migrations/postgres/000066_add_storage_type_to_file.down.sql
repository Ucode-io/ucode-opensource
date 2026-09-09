ALTER TABLE "file"
    DROP COLUMN IF EXISTS "storage_type";

DROP TYPE IF EXISTS file_storage_type;
