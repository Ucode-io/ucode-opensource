INSERT INTO field("id", "table_id", "required", "slug", "label", "default", "type", "index", "attributes", "is_visible", "is_system", "is_search", "autofill_field", "autofill_table", "relation_id", "unique", "automatic") VALUES
('a0557a9c-b916-4133-9e77-4091924469a5', '1b066143-9aad-4b28-bd34-0032709e463b', false, 'menu_drag', 'Menu drag', 'true', 'SWITCH', 'string', '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', false, true, true, '', '', NULL, false, false)
ON CONFLICT (table_id, slug) DO NOTHING;

ALTER TABLE IF EXISTS "global_permission"
    ADD COLUMN IF NOT EXISTS "menu_drag" BOOLEAN DEFAULT true;

UPDATE "global_permission"
SET "menu_drag" = true
WHERE "menu_drag" IS NULL;
