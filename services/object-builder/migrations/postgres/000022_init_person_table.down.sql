-- DELETE relation
-- DELETE relations
DELETE FROM "relation" 
WHERE "table_from" = 'person';

-- DELETE relation fields
DELETE FROM "field" 
WHERE "slug" = 'client_type_id' 
    AND "table_id" = 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5';

DELETE FROM "field" 
WHERE "slug" = 'role_id' 
    AND "table_id" = 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5';

-- DELETE relation views
DELETE FROM "view" 
WHERE "relation_id" = '2da35b3e-1f94-46e6-aa6a-f5ead4d7bb37';

DELETE FROM "view" 
WHERE "relation_id" = 'a94917b8-782d-4355-aec2-d65d7efe2630';

-- DELETE fields
DELETE FROM "field" 
WHERE "id" IN (
        '4f7ade49-da8a-4534-b3a4-35f2875609b1', 
        '88ef053a-ae80-44a0-aad1-055f4405a3ee', 
        '4342bf9d-24ad-4f74-bb7e-156d3b4e1dfd'
);

-- DELETE field permissions
DELETE FROM "field_permission" 
WHERE "field_id" IN (
        '4f7ade49-da8a-4534-b3a4-35f2875609b1', 
        '88ef053a-ae80-44a0-aad1-055f4405a3ee', 
        '4342bf9d-24ad-4f74-bb7e-156d3b4e1dfd', 
        '2b1549b1-5490-41f3-8442-d3e116d847fb', 
        '84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a'
);

-- DROP fields columns
ALTER TABLE "person" 
        DROP COLUMN IF EXISTS "client_type_id",
        DROP COLUMN IF EXISTS "role_id",
        DROP COLUMN IF EXISTS "login",
        DROP COLUMN IF EXISTS "password",
        DROP COLUMN IF EXISTS "user_id_auth";

-- DELETE table view
DELETE FROM "view" 
WHERE "id" = '0db9b1a2-00cd-4ce0-897c-5a71a764639a';

-- DELETE layout, tab, and sections
DELETE FROM "layout" 
WHERE "id" = '371f662a-3a44-46be-a9fa-92add32b27aa';

DELETE FROM "tab" 
WHERE "layout_id" = '371f662a-3a44-46be-a9fa-92add32b27aa';

DELETE FROM "section" 
WHERE "tab_id" = '7d375dd3-d291-4865-8134-4be46b2ce0a5';
