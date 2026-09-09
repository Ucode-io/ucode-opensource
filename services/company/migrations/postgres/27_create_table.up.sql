CREATE TABLE IF NOT EXISTS menu_templates (
    "id" UUID PRIMARY KEY,
    "background" VARCHAR,
    "active_background" VARCHAR,
    "text" VARCHAR,
    "active_text" VARCHAR,
    "title" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);