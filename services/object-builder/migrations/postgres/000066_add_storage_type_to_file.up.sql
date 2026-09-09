DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'file_storage_type') THEN
        CREATE TYPE file_storage_type AS ENUM ('minio', 'google_drive');
    END IF;
END $$;

ALTER TABLE "file"
    ADD COLUMN IF NOT EXISTS "storage_type" file_storage_type NOT NULL DEFAULT 'minio';
