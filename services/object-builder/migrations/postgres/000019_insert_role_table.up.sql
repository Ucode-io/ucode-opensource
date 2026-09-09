INSERT INTO field (
    id, table_id, required, slug, label, "default", type, index, attributes,
    is_visible, is_system, is_search, created_at, updated_at
) VALUES (
    'dd1cce54-2333-4556-97ab-3663c577a28c', -- id
    '1ab7fadc-1f2b-4934-879d-4e99772526ad', -- table_id
    false,                                  -- required
    'status',                               -- slug
    'Статус',                               -- label
    '',                                     -- default
    'SWITCH',                               -- type
    'string',                               -- index
    '{"defaultValue": "TRUE", "label": "", "label_en": "Статус", "number_of_rounds": null}'::jsonb, -- attributes
    true,                                   -- is_visible
    true,                                   -- is_system
    false,                                  -- is_search
    CURRENT_TIMESTAMP,                      -- created_at
    CURRENT_TIMESTAMP                       -- updated_at
)
ON CONFLICT (id) 
DO UPDATE SET 
    required = EXCLUDED.required,
    label = EXCLUDED.label,
    "default" = EXCLUDED."default",
    type = EXCLUDED.type,
    index = EXCLUDED.index,
    attributes = EXCLUDED.attributes,
    is_visible = EXCLUDED.is_visible,
    is_system = EXCLUDED.is_system,
    is_search = EXCLUDED.is_search,
    updated_at = CURRENT_TIMESTAMP;