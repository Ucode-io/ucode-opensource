-- New fare_items: builders, projects, tokens_day, tokens_month
-- builders     id = b1d849e1-7c06-4281-9f36-7f01778c6f23
-- projects     id = e7296d25-163a-4116-a392-9b14a4c7add2
-- tokens_day   id = 6482ece4-31cf-4d7d-8338-61673a1e2efb
-- tokens_month id = ebc2c55d-b61f-4db3-94f3-57edc2ab834b
INSERT INTO fare_item (id, name, parent_id, info, type, created_at, updated_at, deleted_at) VALUES
('b1d849e1-7c06-4281-9f36-7f01778c6f23', 'Builders',           NULL, '', 'builders',     NOW(), NOW(), 0),
('e7296d25-163a-4116-a392-9b14a4c7add2', 'Projects',           NULL, '', 'projects',     NOW(), NOW(), 0),
('6482ece4-31cf-4d7d-8338-61673a1e2efb', 'AI Credits / Day',   NULL, '', 'tokens_day',   NOW(), NOW(), 0),
('ebc2c55d-b61f-4db3-94f3-57edc2ab834b', 'AI Credits / Month', NULL, '', 'tokens_month', NOW(), NOW(), 0)
ON CONFLICT (id) DO NOTHING;

-- ugen fares: Free, Basic, Pro (product_type = 'ugen')
-- ugen_free  id = 07d8a364-ebb2-4291-a452-f44b335cb031
-- ugen_basic id = 26b264b7-abad-4813-87c0-0d95e6cf226a
-- ugen_pro   id = 16decb16-1939-4502-a219-1cfd44480442
INSERT INTO fare (id, name, price, trial_days, disactivate_day, description, is_public, created_at, updated_at, deleted_at, currency_id, product_type) VALUES
('07d8a364-ebb2-4291-a452-f44b335cb031', 'Free',  0.00,   0,  0, 'Free tier for ugen projects.',  TRUE, NOW(), NOW(), 0, '88c816a3-24e8-4994-ab70-9bc826bb9dc3', 'ugen'),
('26b264b7-abad-4813-87c0-0d95e6cf226a', 'Basic', 300.00, 30, 0, 'Basic tier for ugen projects.', TRUE, NOW(), NOW(), 0, '88c816a3-24e8-4994-ab70-9bc826bb9dc3', 'ugen'),
('16decb16-1939-4502-a219-1cfd44480442', 'Pro',   600.00, 30, 0, 'Pro tier for ugen projects.',   TRUE, NOW(), NOW(), 0, '88c816a3-24e8-4994-ab70-9bc826bb9dc3', 'ugen')
ON CONFLICT (id) DO NOTHING;

-- fare_item_price for ugen Free / Basic / Pro (14 items each)
-- Existing fare_item IDs reused:
--   users_count       = f3cb57f8-0571-4775-b819-c46f16357e59
--   items             = 0d6fda97-c8fe-4c8e-b949-2853268f95d2
--   tables            = 95fc7ebf-2a79-40af-b20c-0b9d53fc51f5
--   api_keys          = 3f6ee755-6ad5-4c50-8970-17eeab3f086c
--   request_per_month = 8a79412b-f41b-4299-a42a-9ee359b504d4
--   request_per_second= 3dbbf052-00bf-4158-b0c2-257ff5f34c17
--   function          = 9ef8386b-b15c-4172-807c-c7b8e40df1ff
--   microfrontend     = dbc4c105-abef-4879-a015-136cd713a203
--   database          = 4f521523-0cc8-4d13-9823-e43ced2ca1df
--   asset_size        = 1873b655-af22-40c8-a94e-ff577f818bc5
INSERT INTO fare_item_price (id, fare_id, fare_item_id, value, price, over_limit_price, created_at, updated_at, deleted_at, item_type)
SELECT vals.id::uuid, vals.fare_id::uuid, vals.fare_item_id::uuid, vals.value, vals.price::decimal, vals.over_limit_price::decimal, NOW(), NOW(), 0, vals.item_type::fare_item_type
FROM (VALUES
    -- ugen Free
    ('99baf590-99b5-4c37-9fdd-ec287158393b', '07d8a364-ebb2-4291-a452-f44b335cb031', 'b1d849e1-7c06-4281-9f36-7f01778c6f23', '1',      '0.00', '0.00', 'builders'),
    ('70b42dca-ff20-4bb4-bcd5-bf1f8af6e1ee', '07d8a364-ebb2-4291-a452-f44b335cb031', 'e7296d25-163a-4116-a392-9b14a4c7add2', '1',      '0.00', '0.00', 'projects'),
    ('81c4132c-4451-4834-9bc6-adcaa2c8e50b', '07d8a364-ebb2-4291-a452-f44b335cb031', 'f3cb57f8-0571-4775-b819-c46f16357e59', '1',      '0.00', '0.00', 'users_count'),
    ('cd573528-e35c-453a-8668-c37de911ea17', '07d8a364-ebb2-4291-a452-f44b335cb031', 'ebc2c55d-b61f-4db3-94f3-57edc2ab834b', '50000',  '0.00', '0.00', 'tokens_month'),
    ('f62f8413-79be-41e2-bc74-6a75e152256a', '07d8a364-ebb2-4291-a452-f44b335cb031', '6482ece4-31cf-4d7d-8338-61673a1e2efb', '1700',   '0.00', '0.00', 'tokens_day'),
    ('cc3e25c0-71ee-4db6-bc78-a1ab54970895', '07d8a364-ebb2-4291-a452-f44b335cb031', '8a79412b-f41b-4299-a42a-9ee359b504d4', '100000', '0.00', '0.00', 'request_per_month'),
    ('55288de5-8bb2-45a7-bf1a-c3cf439dabb3', '07d8a364-ebb2-4291-a452-f44b335cb031', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '100',    '0.00', '0.00', 'request_per_second'),
    ('d964d4a4-c268-4b66-ac28-5b55be53e03a', '07d8a364-ebb2-4291-a452-f44b335cb031', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '1',      '0.00', '0.00', 'function'),
    ('4878972a-4693-4073-92e4-f94c95fb0e22', '07d8a364-ebb2-4291-a452-f44b335cb031', 'dbc4c105-abef-4879-a015-136cd713a203', '1',      '0.00', '0.00', 'microfrontend'),
    ('0a5ccaaf-90f2-436a-9869-fd0b3ce80b9a', '07d8a364-ebb2-4291-a452-f44b335cb031', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '10GB',   '0.00', '0.00', 'database'),
    ('a49f6d32-a66b-48ef-b3f7-250f5d354326', '07d8a364-ebb2-4291-a452-f44b335cb031', '1873b655-af22-40c8-a94e-ff577f818bc5', '10GB',   '0.00', '0.00', 'asset_size'),
    ('8ac5e4bb-86c0-445a-a08f-66471e901858', '07d8a364-ebb2-4291-a452-f44b335cb031', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '10000',  '0.00', '0.00', 'items'),
    ('007f2588-a044-4770-9a4e-14bf4f1e3391', '07d8a364-ebb2-4291-a452-f44b335cb031', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '50',     '0.00', '0.00', 'tables'),
    ('c1f2ba75-a989-4efb-b582-fce301b16e91', '07d8a364-ebb2-4291-a452-f44b335cb031', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '2',      '0.00', '0.00', 'api_keys'),
    -- ugen Basic
    ('414cd2f5-ceb7-4a00-ae93-82c452fe19c1', '26b264b7-abad-4813-87c0-0d95e6cf226a', 'b1d849e1-7c06-4281-9f36-7f01778c6f23', '2',      '0.00', '0.00', 'builders'),
    ('55140dec-e59c-4ee6-9a91-2afa11b0469d', '26b264b7-abad-4813-87c0-0d95e6cf226a', 'e7296d25-163a-4116-a392-9b14a4c7add2', '5',      '0.00', '0.00', 'projects'),
    ('66d1507b-ec51-42a2-bab7-ce73c5a1f204', '26b264b7-abad-4813-87c0-0d95e6cf226a', 'f3cb57f8-0571-4775-b819-c46f16357e59', '1000',   '0.00', '0.00', 'users_count'),
    ('c0476af0-e320-4ff2-b558-dc8f219d4841', '26b264b7-abad-4813-87c0-0d95e6cf226a', 'ebc2c55d-b61f-4db3-94f3-57edc2ab834b', '500000', '0.00', '0.00', 'tokens_month'),
    ('d48f539c-e79b-4a35-bd93-eadefa567dca', '26b264b7-abad-4813-87c0-0d95e6cf226a', '6482ece4-31cf-4d7d-8338-61673a1e2efb', '17000',  '0.00', '0.00', 'tokens_day'),
    ('a070de32-331e-4d0b-81a7-ca7cdf4755ff', '26b264b7-abad-4813-87c0-0d95e6cf226a', '8a79412b-f41b-4299-a42a-9ee359b504d4', '250000', '0.00', '0.00', 'request_per_month'),
    ('60f84a44-0e3d-4467-95d9-f0f898f11640', '26b264b7-abad-4813-87c0-0d95e6cf226a', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '200',    '0.00', '0.00', 'request_per_second'),
    ('85fb0bc5-c4dd-435b-b40a-abdce59c4e59', '26b264b7-abad-4813-87c0-0d95e6cf226a', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '10',     '0.00', '0.00', 'function'),
    ('d828efe6-4f03-4a9c-a800-7f66ace82458', '26b264b7-abad-4813-87c0-0d95e6cf226a', 'dbc4c105-abef-4879-a015-136cd713a203', '5',      '0.00', '0.00', 'microfrontend'),
    ('162da33d-ceb3-48ca-95c1-e5a4fbf8246d', '26b264b7-abad-4813-87c0-0d95e6cf226a', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '50GB',   '0.00', '0.00', 'database'),
    ('9133f555-4595-495d-b826-f1bfae2b8521', '26b264b7-abad-4813-87c0-0d95e6cf226a', '1873b655-af22-40c8-a94e-ff577f818bc5', '50GB',   '0.00', '0.00', 'asset_size'),
    ('34b472cd-a4bc-437f-930f-aed861ba04d3', '26b264b7-abad-4813-87c0-0d95e6cf226a', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '100000', '0.00', '0.00', 'items'),
    ('cb59d63c-7cb0-4d74-a9a0-312f434c88e3', '26b264b7-abad-4813-87c0-0d95e6cf226a', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '200',    '0.00', '0.00', 'tables'),
    ('8f8c90fe-8a0e-46ac-ac40-d98934030479', '26b264b7-abad-4813-87c0-0d95e6cf226a', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '10',     '0.00', '0.00', 'api_keys'),
    -- ugen Pro
    ('446d740b-ad22-4a15-96fa-1562fd49a1d1', '16decb16-1939-4502-a219-1cfd44480442', 'b1d849e1-7c06-4281-9f36-7f01778c6f23', '5',       '0.00', '0.00', 'builders'),
    ('318a987a-0bcb-4a58-b7cb-02993d1c57b0', '16decb16-1939-4502-a219-1cfd44480442', 'e7296d25-163a-4116-a392-9b14a4c7add2', '10',      '0.00', '0.00', 'projects'),
    ('b55cfe49-78c3-4f6e-a84b-b3d74d7fc922', '16decb16-1939-4502-a219-1cfd44480442', 'f3cb57f8-0571-4775-b819-c46f16357e59', '10000',   '0.00', '0.00', 'users_count'),
    ('c03bf62a-c075-4cb5-b838-cd70ba7017cf', '16decb16-1939-4502-a219-1cfd44480442', 'ebc2c55d-b61f-4db3-94f3-57edc2ab834b', '1000000', '0.00', '0.00', 'tokens_month'),
    ('28e8721f-2cdb-460f-ae5f-18080a7db49e', '16decb16-1939-4502-a219-1cfd44480442', '6482ece4-31cf-4d7d-8338-61673a1e2efb', '33000',   '0.00', '0.00', 'tokens_day'),
    ('bc27385d-709e-49c8-aca6-a79dc7162b3a', '16decb16-1939-4502-a219-1cfd44480442', '8a79412b-f41b-4299-a42a-9ee359b504d4', '500000',  '0.00', '0.00', 'request_per_month'),
    ('7f3e477b-5946-4c84-8afe-efca5807be8a', '16decb16-1939-4502-a219-1cfd44480442', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '500',     '0.00', '0.00', 'request_per_second'),
    ('b66f5332-6f6a-4787-b11a-ee40d20a25a7', '16decb16-1939-4502-a219-1cfd44480442', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '20',      '0.00', '0.00', 'function'),
    ('ce3f5539-f6c9-43ae-be2d-da001e4441af', '16decb16-1939-4502-a219-1cfd44480442', 'dbc4c105-abef-4879-a015-136cd713a203', '10',      '0.00', '0.00', 'microfrontend'),
    ('a08683ac-1629-45e7-a8c1-36a69354f1a6', '16decb16-1939-4502-a219-1cfd44480442', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '100GB',   '0.00', '0.00', 'database'),
    ('53fc9acd-8994-43a9-ae33-2ebc26b1c8d9', '16decb16-1939-4502-a219-1cfd44480442', '1873b655-af22-40c8-a94e-ff577f818bc5', '100GB',   '0.00', '0.00', 'asset_size'),
    ('1f22d995-8dd1-46d0-a45d-f1ca8422fe81', '16decb16-1939-4502-a219-1cfd44480442', '0d6fda97-c8fe-4c8e-b949-2853268f95d2', '1000000', '0.00', '0.00', 'items'),
    ('65802dcd-c368-429a-9ff1-6f0828404744', '16decb16-1939-4502-a219-1cfd44480442', '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5', '500',     '0.00', '0.00', 'tables'),
    ('7a95cdca-b666-442f-98c2-688cd5148de3', '16decb16-1939-4502-a219-1cfd44480442', '3f6ee755-6ad5-4c50-8970-17eeab3f086c', '25',      '0.00', '0.00', 'api_keys')
) AS vals(id, fare_id, fare_item_id, value, price, over_limit_price, item_type)
JOIN fare f ON f.id = vals.fare_id::uuid
ON CONFLICT (fare_id, item_type) DO NOTHING;