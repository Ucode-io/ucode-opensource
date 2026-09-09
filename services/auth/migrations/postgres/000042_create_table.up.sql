BEGIN;

    CREATE TABLE IF NOT EXISTS "client_platform" (
        "id" UUID PRIMARY KEY,
        "name" VARCHAR NOT NULL,
        "subdomain" VARCHAR,
        "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
        "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
    );

    INSERT INTO "client_platform"("id", "name", "subdomain") 
    VALUES 
        ('7d4a4c38-dd84-4902-b744-0488b80a4c01', 'ADMIN PANEL', 'app.u-code.io'),
        ('7d4a4c38-dd84-4902-b744-0488b80a4c02', 'MOBILE APP', ''),
        ('7d4a4c38-dd84-4902-b744-0488b80a4c03', 'WEBSITE', 'u-code.io'),
        ('7d4a4c38-dd84-4902-b744-0488b80a4c04', 'AUTHORIZATION CODE', 'ofs.u-code.io')
    ON CONFLICT ("id") DO NOTHING;

    ALTER TABLE IF EXISTS "api_keys" 
        ADD COLUMN IF NOT EXISTS "client_platform_id" UUID REFERENCES "client_platform"(id) DEFAULT '7d4a4c38-dd84-4902-b744-0488b80a4c04',
        ADD COLUMN IF NOT EXISTS "client_id" VARCHAR,
        ADD COLUMN IF NOT EXISTS "disable" BOOLEAN DEFAULT FALSE;

    CREATE UNIQUE INDEX IF NOT EXISTS api_keys_client_id_uindex ON api_keys (client_id)
        WHERE client_id IS NOT NULL;

    ALTER TABLE IF EXISTS "session" 
        ADD COLUMN IF NOT EXISTS "client_id" VARCHAR;

    UPDATE "api_keys" SET "disable" = TRUE;

    CREATE TABLE IF NOT EXISTS "client_tokens" (
        id UUID PRIMARY KEY,
        client_id VARCHAR NOT NULL,
        info JSONB DEFAULT '{}',
        given_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
    );

COMMIT;
