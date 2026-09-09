INSERT INTO field("id", "table_id", "required", "slug", "label", "default", "type", "index", "attributes", "is_visible", "is_system", "is_search", "autofill_field", "autofill_table", "relation_id", "unique", "automatic")
SELECT 
    '07a21e20-d3a2-41ce-b7af-6b1d7e5d2d59', 
    '25698624-5491-4c39-99ec-aed2eaf07b97', 
    false, 
    'search_button', 
    'Search Button', 
    '', 
    'SINGLE_LINE', 
    'string', 
    '{"fields":{"maxLength":{"kind":"stringValue","stringValue":""},"placeholder":{"kind":"stringValue","stringValue":""},"showTooltip":{"boolValue":false,"kind":"boolValue"}}}', 
    false, 
    true, 
    true, 
    '', 
    '', 
    NULL, 
    false, 
    false
WHERE NOT EXISTS (
    SELECT 1 
    FROM field 
    WHERE "id" = '07a21e20-d3a2-41ce-b7af-6b1d7e5d2d59' 
      AND "table_id" = '25698624-5491-4c39-99ec-aed2eaf07b97'
);
