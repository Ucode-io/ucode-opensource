-- Group fare_items (parent nodes). type='custom', identified by parent_id references.
-- workspace      id = e903f936-b697-4037-9167-0f8158530080
-- api            id = e597ef26-b95c-4d20-9299-72414736da3b
-- infrastructure id = c64cdc9a-710d-4982-a9e1-2fda0b2f2766
INSERT INTO fare_item (id, name, parent_id, info, type, created_at, updated_at, deleted_at) VALUES
('e903f936-b697-4037-9167-0f8158530080', 'Workspace',      NULL, 'group', 'custom', NOW(), NOW(), 0),
('e597ef26-b95c-4d20-9299-72414736da3b', 'API',            NULL, 'group', 'custom', NOW(), NOW(), 0),
('c64cdc9a-710d-4982-a9e1-2fda0b2f2766', 'Infrastructure', NULL, 'group', 'custom', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

-- WORKSPACE: builders, projects, users_count, tokens_month, tokens_day
UPDATE fare_item SET parent_id = 'e903f936-b697-4037-9167-0f8158530080'
WHERE id IN (
    'b1d849e1-7c06-4281-9f36-7f01778c6f23',
    'e7296d25-163a-4116-a392-9b14a4c7add2',
    'f3cb57f8-0571-4775-b819-c46f16357e59',
    'ebc2c55d-b61f-4db3-94f3-57edc2ab834b',
    '6482ece4-31cf-4d7d-8338-61673a1e2efb'
);

-- API: request_per_month, request_per_second
UPDATE fare_item SET parent_id = 'e597ef26-b95c-4d20-9299-72414736da3b'
WHERE id IN (
    '8a79412b-f41b-4299-a42a-9ee359b504d4',
    '3dbbf052-00bf-4158-b0c2-257ff5f34c17'
);

-- INFRASTRUCTURE: function, microfrontend, database, asset_size, items, tables, api_keys
UPDATE fare_item SET parent_id = 'c64cdc9a-710d-4982-a9e1-2fda0b2f2766'
WHERE id IN (
    '9ef8386b-b15c-4172-807c-c7b8e40df1ff',
    'dbc4c105-abef-4879-a015-136cd713a203',
    '4f521523-0cc8-4d13-9823-e43ced2ca1df',
    '1873b655-af22-40c8-a94e-ff577f818bc5',
    '0d6fda97-c8fe-4c8e-b949-2853268f95d2',
    '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5',
    '3f6ee755-6ad5-4c50-8970-17eeab3f086c'
);
