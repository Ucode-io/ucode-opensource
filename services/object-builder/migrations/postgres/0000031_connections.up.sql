CREATE TABLE IF NOT EXISTS "tracked_connections" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(63),
    "connection_string" TEXT NOT NULL UNIQUE,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "tracked_tables" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "connection_id" UUID REFERENCES tracked_connections(id) ON DELETE SET NULL,
    "table_name" VARCHAR(63) NOT NULL,
    "is_tracked" BOOLEAN DEFAULT FALSE,
    "fields" JSONB NOT NULL DEFAULT '[]',
    "relations" JSONB NOT NULL DEFAULT '[]',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (connection_id, table_name)
);