CREATE TABLE IF NOT EXISTS "template" (
    "id" UUID PRIMARY KEY,
    "name" VARCHAR NOT NULL,
    "description" TEXT,
    "photo" VARCHAR,
    "tables" JSONB NOT NULL DEFAULT '{}', --field, relation, view, action, menu, layout, section, tab
    "functions" JSONB NOT NULL DEFAULT '{}',
    "microfronts" JSONB NOT NULL DEFAULT '{}',
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" BIGINT DEFAULT 0 NOT NULL
);