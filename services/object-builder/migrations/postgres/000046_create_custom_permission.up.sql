CREATE TABLE IF NOT EXISTS "custom_permission" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "parent_id" UUID REFERENCES "custom_permission"("id") ON DELETE SET NULL,
    "title" VARCHAR(255) NOT NULL,
    "attributes" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "custom_permission_access" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "custom_permission_id" UUID NOT NULL REFERENCES "custom_permission"("id") ON DELETE CASCADE,
    "role_id" UUID NOT NULL REFERENCES "role"("guid") ON DELETE CASCADE,
    "client_type_id" UUID NOT NULL REFERENCES "client_type"("guid") ON DELETE CASCADE,

    "read" VARCHAR(3) NOT NULL DEFAULT 'No' CHECK ("read" IN ('Yes', 'No')),
    "write" VARCHAR(3) NOT NULL DEFAULT 'No' CHECK ("write" IN ('Yes', 'No')),
    "update" VARCHAR(3) NOT NULL DEFAULT 'No' CHECK ("update" IN ('Yes', 'No')),
    "delete" VARCHAR(3) NOT NULL DEFAULT 'No' CHECK ("delete" IN ('Yes', 'No')),

    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE("custom_permission_id", "role_id", "client_type_id")
);