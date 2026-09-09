ALTER TABLE IF EXISTS "view" 
    ADD COLUMN IF NOT EXISTS "menu_id" UUID REFERENCES "menu"("id") ON DELETE SET NULL,
    DROP COLUMN IF EXISTS "multiple_insert_field",
    DROP COLUMN IF EXISTS "main_field";

UPDATE 
    view v
SET menu_id = m.id
FROM "table" t
JOIN menu m ON m.table_id = t.id
WHERE t.slug = v.table_slug;