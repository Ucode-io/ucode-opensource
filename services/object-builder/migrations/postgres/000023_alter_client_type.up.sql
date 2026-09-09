ALTER TABLE "client_type"
ADD COLUMN IF NOT EXISTS "session_limit" INTEGER DEFAULT 50;

INSERT INTO "field" (
        "id", 
        "table_id", 
        "required", 
        "slug", 
        "label", 
        "default", 
        "type", 
        "index", 
        "attributes", 
        "is_visible", 
        "is_system", 
        "is_search", 
        "autofill_field", 
        "autofill_table", 
        "relation_id", 
        "unique", 
        "automatic"
    ) VALUES 
(
    '4cb3bfc7-7160-4d36-b71a-e0878f368ae8', 
    'ed3bf0d9-40a3-4b79-beb4-52506aa0b5ea', 
    false, 
    'session_limit', 
    'Session Limit', 
    '', 
    'NUMBER', 
    'string', 
    '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', 
    false, true, true, '', '', NULL, false, false
);