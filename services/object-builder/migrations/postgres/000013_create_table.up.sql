CREATE TABLE IF NOT EXISTS "docx_templates" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" UUID,
    "title" VARCHAR(255),
    "table_slug" VARCHAR(255),
    "file_url" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);