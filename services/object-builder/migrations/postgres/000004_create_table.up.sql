CREATE TABLE IF NOT EXISTS "menu_templates" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "background" VARCHAR(255),
    "active_background" VARCHAR(255),
    "text" VARCHAR(255),
    "active_text" VARCHAR(255),
    "title" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);