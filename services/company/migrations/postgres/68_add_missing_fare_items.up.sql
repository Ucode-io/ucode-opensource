-- Fix 'Asset Size info' fare_item: was incorrectly typed as 'database', now 'asset_size'
UPDATE fare_item SET type = 'asset_size' WHERE id = '1873b655-af22-40c8-a94e-ff577f818bc5';

-- New fare_items
-- users_count id = f3cb57f8-0571-4775-b819-c46f16357e59
-- items       id = 0d6fda97-c8fe-4c8e-b949-2853268f95d2
-- tables      id = 95fc7ebf-2a79-40af-b20c-0b9d53fc51f5
-- api_keys    id = 3f6ee755-6ad5-4c50-8970-17eeab3f086c
INSERT INTO fare_item (id, name, parent_id, info, type, created_at, updated_at, deleted_at) VALUES
('f3cb57f8-0571-4775-b819-c46f16357e59', 'Users Count', NULL, '', 'users_count', NOW(), NOW(), 0),
('0d6fda97-c8fe-4c8e-b949-2853268f95d2', 'Items',       NULL, '', 'items',       NOW(), NOW(), 0),
('95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', 'Tables',      NULL, '', 'tables',      NOW(), NOW(), 0),
('3f6ee755-6ad5-4c50-8970-17eeab3f086c', 'API Keys',    NULL, '', 'api_keys',    NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

-- fare_item_price for each fare (JOIN fare to skip missing fares safely)
-- Small   (7d958abe): asset_size=50GB, users_count=0(∞), items=0(∞), tables=0(∞), api_keys=0(∞)
-- Medium  (8c602ff5): asset_size=100GB, users_count=0(∞), items=0(∞), tables=0(∞), api_keys=0(∞)
-- Private (81c98c75): asset_size=50GB, users_count=0(∞), items=0(∞), tables=0(∞), api_keys=0(∞)
INSERT INTO fare_item_price (id, fare_id, fare_item_id, value, price, over_limit_price, created_at, updated_at, deleted_at, item_type)
SELECT vals.id::uuid, vals.fare_id::uuid, vals.fare_item_id::uuid, vals.value, vals.price::decimal, vals.over_limit_price::decimal, NOW(), NOW(), 0, vals.item_type::fare_item_type
FROM (VALUES
    -- asset_size
    ('61bb7c07-9c84-4de0-8c99-b9ed8bc4c1a4', '7d958abe-08df-4735-a4f3-d275d1482b47', '1873b655-af22-40c8-a94e-ff577f818bc5', '50GB',  '0.00', '0.00', 'asset_size'),
    ('9c04ff8c-f635-492c-8620-44b10bad942b', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '1873b655-af22-40c8-a94e-ff577f818bc5', '100GB', '0.00', '0.00', 'asset_size'),
    ('af4c7be0-489c-4a06-b09f-1db4458adbc6', '81c98c75-8bf3-41fe-8401-c79589809750', '1873b655-af22-40c8-a94e-ff577f818bc5', '50GB',  '0.00', '0.00', 'asset_size'),
    -- users_count (unlimited)
    ('9947c85d-cd52-45c7-aac8-ae6285975c6b', '7d958abe-08df-4735-a4f3-d275d1482b47', 'f3cb57f8-0571-4775-b819-c46f16357e59', '0', '0.00', '0.00', 'users_count'),
    ('36dec5c1-4dc1-4c53-8905-edd19fa6423a', '8c602ff5-fd97-429a-870c-a6ebd46832c5', 'f3cb57f8-0571-4775-b819-c46f16357e59', '0', '0.00', '0.00', 'users_count'),
    ('d2b062e1-0e9f-4a75-878c-52feef69c2d1', '81c98c75-8bf3-41fe-8401-c79589809750', 'f3cb57f8-0571-4775-b819-c46f16357e59', '0', '0.00', '0.00', 'users_count'),
    -- items (unlimited)
    ('e161ce1e-1712-481c-a7b0-b01660037742', '7d958abe-08df-4735-a4f3-d275d1482b47', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '0', '0.00', '0.00', 'items'),
    ('6c6227dc-3a24-45bb-aa72-96cc04fc3af5', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '0', '0.00', '0.00', 'items'),
    ('f6d1fc0e-2841-4813-bdcc-171525b16061', '81c98c75-8bf3-41fe-8401-c79589809750', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '0', '0.00', '0.00', 'items'),
    -- tables (unlimited)
    ('7093ece2-4977-4447-ac69-f46460caad46', '7d958abe-08df-4735-a4f3-d275d1482b47', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '0', '0.00', '0.00', 'tables'),
    ('0d18ebc4-7628-472c-aba5-e71eb4197f75', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '0', '0.00', '0.00', 'tables'),
    ('0e6826d0-f2f3-43cb-a6fc-2d87d13b9679', '81c98c75-8bf3-41fe-8401-c79589809750', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '0', '0.00', '0.00', 'tables'),
    -- api_keys (unlimited)
    ('a349bc04-965d-4ed0-a2c4-20c97578a8ea', '7d958abe-08df-4735-a4f3-d275d1482b47', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '0', '0.00', '0.00', 'api_keys'),
    ('3262c8b8-adfe-4bbd-9c6a-2b9b2de1951a', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '0', '0.00', '0.00', 'api_keys'),
    ('89d0fa1d-ae55-4398-b534-d31ff5c65ff0', '81c98c75-8bf3-41fe-8401-c79589809750', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '0', '0.00', '0.00', 'api_keys')
) AS vals(id, fare_id, fare_item_id, value, price, over_limit_price, item_type)
JOIN fare f ON f.id = vals.fare_id::uuid
ON CONFLICT (fare_id, item_type) DO NOTHING;
