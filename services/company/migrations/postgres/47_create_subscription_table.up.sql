CREATE TABLE IF NOT EXISTS discount (
    id                  UUID PRIMARY KEY,
    months              INTEGER NOT NULL DEFAULT 1,
    value               DECIMAL(5,2) NOT NULL DEFAULT 0,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscription (
    id              UUID PRIMARY KEY,
    project_id      UUID REFERENCES project(id) ON DELETE CASCADE NOT NULL,
    fare_id         UUID REFERENCES fare(id) ON DELETE CASCADE NOT NULL,
    discount_id     UUID REFERENCES discount(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'active' 
                    CHECK (status IN ('active', 'canceled', 'paused', 'expired')),
    start_date      DATE NOT NULL DEFAULT CURRENT_DATE,
    end_date        DATE NOT NULL, 
    renewal_date    DATE NOT NULL,
    auto_renew      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE IF EXISTS "transaction"
    ADD COLUMN IF NOT EXISTS subscription_id UUID REFERENCES subscription(id) ON DELETE CASCADE;

ALTER TABLE IF EXISTS fare
    DROP COLUMN IF EXISTS currency,
    ADD COLUMN IF NOT EXISTS currency_id UUID REFERENCES currency(id) ON DELETE SET NULL DEFAULT '88c816a3-24e8-4994-ab70-9bc826bb9dc3';

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'project_status'
    ) THEN
        ALTER TYPE project_status ADD VALUE IF NOT EXISTS 'insufficient_funds';
    END IF;
END;
$$;

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'subscription';
    END IF;
END;
$$;

-- FARE 
INSERT INTO fare (
    id, name, price, trial_days, disactivate_day, description, 
    is_public, created_at, updated_at, deleted_at, currency_id
) VALUES
(
    '8c602ff5-fd97-429a-870c-a6ebd46832c5', 'Medium', 600.00, 30, 0, 
    'Advanced features and reporting, better workflows and automation.',
    TRUE, '2025-01-23 09:33:25.344407', '2025-01-24 09:33:25.344407', 0,
    '88c816a3-24e8-4994-ab70-9bc826bb9dc3'
),
(
    '7d958abe-08df-4735-a4f3-d275d1482b47', 'Small', 300.00, 30, 0, 
    'Basic features for up to 10 employees with everything you need.',
    TRUE, '2025-01-24 09:33:25.344407', '2025-01-24 09:33:25.344407', 0,
    '88c816a3-24e8-4994-ab70-9bc826bb9dc3'
),
(
    '81c98c75-8bf3-41fe-8401-c79589809750', 'Private', 100.00, 30, 0, 
    'Basic',
    FALSE, '2025-04-17 10:08:26.950433', '2025-04-17 10:08:26.950433', 0,
    '88c816a3-24e8-4994-ab70-9bc826bb9dc3'
);

-- FARE_ITEM 
INSERT INTO fare_item (id, name, parent_id, info, type, created_at, updated_at, deleted_at) VALUES
('390961e5-ed6e-4f1a-a847-f19650c46b5e', 'Total Users & Roles', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('f6f7a7f6-4421-4c7e-8ce0-aea9f120c28e', 'Configuration', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('8a79412b-f41b-4299-a42a-9ee359b504d4', 'API Requests / Month', NULL, '', 'request_per_month', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('3dbbf052-00bf-4158-b0c2-257ff5f34c17', 'API Requests / Second', NULL, '', 'request_per_second', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('9ef8386b-b15c-4172-807c-c7b8e40df1ff', 'Functions', NULL, '', 'function', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('dbc4c105-abef-4879-a015-136cd713a203', 'Microfrontend', NULL, '', 'microfrontend', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('4f521523-0cc8-4d13-9823-e43ced2ca1df', 'Database Size', NULL, '', 'database', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('1873b655-af22-40c8-a94e-ff577f818bc5', 'Asset Size info', NULL, '', 'database', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('0e371832-550e-4c18-bf4e-7aaed8920e95', 'Database Server', NULL, '', 'database', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('73289b7a-4f74-4ccb-b00a-81cf58cb641c', 'File Server', NULL, '', 'database', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('db25085c-9d09-43d1-aca1-ff8f63031a59', 'Function Server', NULL, '', 'database', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('dea85a8d-f95a-40f9-a132-0b1040b6aee2', 'Microfrontend Server', NULL, '', 'microfrontend', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('4b767519-8184-4082-8ba1-fc5e7e22063d', 'Custom Domain', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('58753da5-a3ca-487a-b93e-51707c9eb78e', 'Failover Instances', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('1c94ace1-2745-4fea-96ed-5d9c0eaed41e', 'Community Support', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('4ef44b21-c498-473e-8216-e2273481b92e', 'Designated Support', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('3435c888-0f7a-4160-84ba-905cf8dbdf12', 'Dedicated Developer', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('17dfabff-6d32-4755-a339-7865f4b8c74d', 'Log retention (API & Database)', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('298681d2-bb81-46b4-827e-b44083a578c2', 'Role Based Access Control', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('beb4dbd0-209d-472e-b82f-10aafe19e47a', 'Menu Permissions', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('f619d75d-0004-45b7-ba5c-67482512ab3f', 'Table Permissions', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('72081d8a-5153-411b-b694-0db3332b3566', 'Field Permissions', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('977c7abc-e7ad-46b1-bf56-f604ccf6ab43', 'REST API', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('2164ed45-a2fe-4cf7-a4e0-d56de1ca1ffc', 'Image & Asset API', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('8ef7777c-59dd-4c20-aa13-a0e249f93d55', 'Web Hooks', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0),
('959d314d-08c3-4268-a3a4-8520c9db7b29', 'Direct Database Access', NULL, '', 'custom', '2025-01-24 09:33:13.857367', '2025-01-24 09:33:13.857367', 0);

-- FARE_ITEM_PRICE
INSERT INTO fare_item_price (
    id, fare_id, fare_item_id, value, price, over_limit_price,
    created_at, updated_at, deleted_at, item_type
) VALUES
('377ae3ce-f07a-49f9-a1cf-5bf378a08982', '7d958abe-08df-4735-a4f3-d275d1482b47', '8a79412b-f41b-4299-a42a-9ee359b504d4', '100000', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'request_per_month'),
('26f70c65-e1d0-4403-9fab-6152d5ea8f0d', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '8a79412b-f41b-4299-a42a-9ee359b504d4', '1000000', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'request_per_month'),
('8edd6bf7-0beb-428d-a7e6-f72ff8069f01', '7d958abe-08df-4735-a4f3-d275d1482b47', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '100', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'request_per_second'),
('78b53631-b72c-4061-90f4-9d10d6072367', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '500', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'request_per_second'),
('63b3a936-5914-40a6-883f-48e63d63a730', '7d958abe-08df-4735-a4f3-d275d1482b47', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '10', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'function'),
('a1178696-f8bb-4f1a-8970-5524ef7795b4', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '10', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'function'),
('7a049411-3083-4304-add9-d15a1b40a46a', '7d958abe-08df-4735-a4f3-d275d1482b47', 'dbc4c105-abef-4879-a015-136cd713a203', '5', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'microfrontend'),
('149431f0-7b58-4c8d-8f71-5a5199f9f8a3', '8c602ff5-fd97-429a-870c-a6ebd46832c5', 'dbc4c105-abef-4879-a015-136cd713a203', '5', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'microfrontend'),
('8807d7a8-b6d2-4a3d-af9c-c0f5d37e9df8', '7d958abe-08df-4735-a4f3-d275d1482b47', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '100GB', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'database'),
('ddc7c930-f4ae-4b8c-918d-99b11e7ebfe4', '8c602ff5-fd97-429a-870c-a6ebd46832c5', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '500GB', 0.00, 0.00, '2025-01-24 10:17:07.406489', '2025-01-24 10:17:07.406489', 0, 'database'),
('028eb48b-eef7-4632-9d08-952f5565f7bf', '81c98c75-8bf3-41fe-8401-c79589809750', '8a79412b-f41b-4299-a42a-9ee359b504d4', '100000', 0.00, 0.00, '2025-04-17 10:08:26.949094', '2025-04-17 10:08:26.949094', 0, 'request_per_month'),
('127bba67-01ce-45ea-b983-4f990e86d410', '81c98c75-8bf3-41fe-8401-c79589809750', '3dbbf052-00bf-4158-b0c2-257ff5f34c17', '100', 0.00, 0.00, '2025-04-17 10:08:26.949094', '2025-04-17 10:08:26.949094', 0, 'request_per_second'),
('15ab7d9a-ad03-463f-bf56-a19e98eef512', '81c98c75-8bf3-41fe-8401-c79589809750', '9ef8386b-b15c-4172-807c-c7b8e40df1ff', '200', 0.00, 0.00, '2025-04-17 10:08:26.949094', '2025-04-17 10:08:26.949094', 0, 'function'),
('40637113-d0db-47a3-9085-9a55a9d0f7d1', '81c98c75-8bf3-41fe-8401-c79589809750', 'dbc4c105-abef-4879-a015-136cd713a203', '5', 0.00, 0.00, '2025-04-17 10:08:26.949094', '2025-04-17 10:08:26.949094', 0, 'microfrontend'),
('676af37c-f154-4738-874e-9c5f3a7fe3db', '81c98c75-8bf3-41fe-8401-c79589809750', '4f521523-0cc8-4d13-9823-e43ced2ca1df', '100GB', 0.00, 0.00, '2025-04-17 10:08:26.949094', '2025-04-17 10:08:26.949094', 0, 'database');
