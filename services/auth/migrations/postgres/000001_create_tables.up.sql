CREATE TABLE IF NOT EXISTS "project" (
    "id" UUID PRIMARY KEY,
    "name" VARCHAR NOT NULL,
    "domain" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS "client_platform" (
    "id" UUID PRIMARY KEY,
    "project_id" UUID NOT NULL,   
    "name" VARCHAR NOT NULL,
    "subdomain" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TYPE "confirm_strategies" AS ENUM ('UNDECIDED', 'PHONE', 'EMAIL');

CREATE TABLE IF NOT EXISTS "client_type" (
    "project_id" UUID NOT NULL,
    "id" UUID PRIMARY KEY,
    "name" VARCHAR,
    "confirm_by" confirm_strategies NOT NULL,
    "self_register" BOOLEAN,
    "self_recover" BOOLEAN,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TYPE "relation_types" AS ENUM ('UNREVEALED', 'BRANCH', 'REGION');

CREATE TABLE IF NOT EXISTS "relation" (
    "id" UUID PRIMARY KEY,
    "client_type_id" UUID REFERENCES "client_type"("id") NOT NULL,
    "type" relation_types NOT NULL,
    "name" VARCHAR NOT NULL,
    "description" TEXT,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS "user_info_field" (
    "id" UUID PRIMARY KEY,
    "client_type_id" UUID REFERENCES "client_type"("id") NOT NULL,
    "field_name" VARCHAR,
    "field_type" VARCHAR,
    "data_type" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TYPE "login_strategies" AS ENUM ('UNKNOWN', 'STANDARD', 'OTP', 'PASSCODE', 'ONE2MANY');

CREATE TABLE IF NOT EXISTS "client" (
    "project_id" UUID DEFAULT 'f5955c82-f264-4655-aeb4-86fd1c642cb6' NOT NULL,
    "client_platform_id" UUID REFERENCES "client_platform"("id"),
    "client_type_id" UUID REFERENCES "client_type"("id"),
    "login_strategy" login_strategies NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY ("client_platform_id", "client_type_id")
);

CREATE TABLE IF NOT EXISTS "role" (
    "project_id" UUID NOT NULL,
    "client_platform_id" UUID REFERENCES "client_platform"("id") NOT NULL,
    "client_type_id" UUID REFERENCES "client_type"("id") NOT NULL,
    "id" UUID PRIMARY KEY,
    "name" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE ("client_type_id", "name")
);

CREATE TABLE IF NOT EXISTS "scope" (
    "client_platform_id" UUID REFERENCES "client_platform"("id"),
    "path" VARCHAR,
    "method" VARCHAR,
    "requests" BIGINT,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY ("client_platform_id", "path", "method")
);

CREATE TABLE IF NOT EXISTS "permission" (
    "id" UUID PRIMARY KEY,
    "client_platform_id" UUID REFERENCES "client_platform"("id") NOT NULL, 
    "parent_id" UUID,
    "name" VARCHAR NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE ("client_platform_id", "parent_id", "name")
);

ALTER TABLE "permission" ADD CONSTRAINT "fk_permission_parent_id" FOREIGN KEY ("parent_id") REFERENCES permission(id);

CREATE TABLE IF NOT EXISTS "permission_scope" (
    "permission_id" UUID REFERENCES "permission"("id"),
    "client_platform_id" UUID,
    "path" VARCHAR,
    "method" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY ("client_platform_id", "path", "method") REFERENCES "scope"("client_platform_id", "path", "method"),
    PRIMARY KEY ("permission_id", "client_platform_id", "path", "method")
);

CREATE TABLE IF NOT EXISTS "role_permission" (
    "role_id" UUID REFERENCES "role"("id"),
    "permission_id" UUID REFERENCES "permission"("id"),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY ("role_id", "permission_id")
);

CREATE TABLE IF NOT EXISTS "integration" (
    "id" UUID PRIMARY KEY,
    "project_id" UUID NOT NULL,
    "client_platform_id" UUID REFERENCES "client_platform"("id"),
    "client_type_id" UUID REFERENCES "client_type"("id"),
    "role_id" UUID REFERENCES "role"("id"),
    "title" VARCHAR,
    "secret_key" VARCHAR(512),
    "ip_whitelist" JSONB,
    "active" SMALLINT,
    "expires_at" TIMESTAMP NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE ("project_id", "client_platform_id")
);

CREATE TABLE IF NOT EXISTS "user" (
    "id" UUID PRIMARY KEY,
    "project_id" UUID NOT NULL,
    "client_platform_id" UUID REFERENCES "client_platform"("id"),
    "client_type_id" UUID REFERENCES "client_type"("id"),
    "role_id" UUID REFERENCES "role"("id"),
    "name" VARCHAR DEFAULT '' NOT NULL,
    "phone" VARCHAR,
    "email" VARCHAR,
    "login" VARCHAR,
    "password" VARCHAR(1000),
    "active" SMALLINT,
    "expires_at" TIMESTAMP NOT NULL,
    "photo_url" VARCHAR DEFAULT '' NOT NULL,
    "passport_serial" VARCHAR(2) DEFAULT '' NOT NULL,
    "passport_number" VARCHAR(7) DEFAULT '' NOT NULL,
    "work_place" VARCHAR(255) DEFAULT '' NOT NULL,
    "salary" VARCHAR(255) DEFAULT '' NOT NULL,
    "verified" BOOLEAN DEFAULT false NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE ("project_id", "client_platform_id", "phone")
    -- UNIQUE ("project_id", "client_platform_id", "email"),
    -- UNIQUE ("project_id", "client_platform_id", "login")
);

CREATE TABLE IF NOT EXISTS "user_relation" (
    "user_id" UUID REFERENCES "user"("id"),
    "relation_id" UUID REFERENCES "relation"("id"),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY ("user_id", "relation_id")
);

CREATE TABLE IF NOT EXISTS "user_info" (
    "user_id" UUID REFERENCES "user"("id"),
    "data" JSONB,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY ("user_id")
);

CREATE TABLE IF NOT EXISTS "session" (
    "id" UUID PRIMARY KEY,
    "project_id" UUID NOT NULL,
    "client_platform_id" UUID REFERENCES "client_platform"("id"),
    "client_type_id" UUID REFERENCES "client_type"("id"),
    "user_id" UUID REFERENCES "user"("id"),
    "role_id" UUID REFERENCES "role"("id"),
    "integration_id" UUID REFERENCES "integration"("id"),
    "ip" INET,
    "data" TEXT,
    "expires_at" TIMESTAMP NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE IF EXISTS "session" ADD CONSTRAINT "integration_id_or_user_id" CHECK (
    ("user_id" IS NULL OR "integration_id" IS NULL) AND NOT
    ("user_id" IS NULL AND "integration_id" IS NULL)
);

CREATE INDEX "idx_session_user_id" ON "session"("user_id");

CREATE TABLE IF NOT EXISTS "passcode" (
    "id" UUID PRIMARY KEY,
    "phone" VARCHAR(15) NOT NULL,
    "hashed_code" VARCHAR(1000) NOT NULL,
    "state" SMALLINT DEFAULT 0,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "expires_at" TIMESTAMP NOT NULL
);
-- CREATE INDEX "idx_passcode_user_id" ON "passcode"("user_id");
