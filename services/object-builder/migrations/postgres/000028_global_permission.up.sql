INSERT INTO field("id", "table_id", "required", "slug", "label", "default", "type", "index", "attributes", "is_visible", "is_system", "is_search", "autofill_field", "autofill_table", "relation_id", "unique", "automatic") VALUES 
('72097726-450e-4a33-92e8-5c0641e1abee', '1b066143-9aad-4b28-bd34-0032709e463b', false, 'gitbook_button', 'Gitbook', '', 'SWITCH', 'string', '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', false, true, true, '', '', NULL, false, false),
('c32e12b3-d7f7-425e-b646-f33a94a25efa', '1b066143-9aad-4b28-bd34-0032709e463b', false, 'chatwoot_button', 'Chatwoot', '', 'SWITCH', 'string', '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', false, true, true, '', '', NULL, false, false),
('8e24b190-413b-433d-bcf1-f1e8e0241f45', '1b066143-9aad-4b28-bd34-0032709e463b', false, 'gpt_button', 'GPT', '', 'SWITCH', 'string', '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', false, true, true, '', '', NULL, false, false);

ALTER TABLE IF EXISTS "global_permission"
ADD COLUMN IF NOT EXISTS "chatwoot_button" BOOLEAN DEFAULT true;

ALTER TABLE IF EXISTS "global_permission"
ADD COLUMN IF NOT EXISTS "gitbook_button" BOOLEAN DEFAULT true;

ALTER TABLE IF EXISTS "global_permission"
ADD COLUMN IF NOT EXISTS "gpt_button" BOOLEAN DEFAULT true;
