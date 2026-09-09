-- Relations
INSERT INTO "relation" (
        "id", "table_from", "table_to", "field_from", "field_to", "type", "view_fields", "relation_field_slug", 
        "is_system", "cascading_tree_table_slug", "cascading_tree_field_slug", "dynamic_tables", "auto_filters"
) VALUES 
(
        '2da35b3e-1f94-46e6-aa6a-f5ead4d7bb37', 'person', 'client_type', 'client_type_id', 
        'id', 'Many2One', '{"04d0889a-b9ba-4f5c-8473-c8447aab350d"}'::text[], '', true, '', '', '{}', 
        '[{}]'::jsonb
),
(
        'a94917b8-782d-4355-aec2-d65d7efe2630', 'person', 'role', 'role_id', 
        'id', 'Many2One', '{"c12adfef-2991-4c6a-9dff-b4ab8810f0df"}'::text[], '', true, '', '', '{}', 
        '[{"field_to": "client_type_id", "field_from": "client_type_id"}]'::jsonb
)
ON CONFLICT (id) DO UPDATE 
SET 
        "table_from" = EXCLUDED."table_from",
        "table_to" = EXCLUDED."table_to",
        "field_from" = EXCLUDED."field_from",
        "field_to" = EXCLUDED."field_to",
        "type" = EXCLUDED."type",
        "view_fields" = EXCLUDED."view_fields",
        "relation_field_slug" = EXCLUDED."relation_field_slug",
        "is_system" = EXCLUDED."is_system",
        "cascading_tree_table_slug" = EXCLUDED."cascading_tree_table_slug",
        "cascading_tree_field_slug" = EXCLUDED."cascading_tree_field_slug",
        "dynamic_tables" = EXCLUDED."dynamic_tables",
        "auto_filters" = EXCLUDED."auto_filters",
        updated_at = NOW();


-- Relation Fields
INSERT INTO "field" ("id", "table_id", "required", "slug", "label", "default", "type", "index", "attributes", "is_system", "relation_id") 
VALUES 
(
    '2b1549b1-5490-41f3-8442-d3e116d847fb', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', true, 
    'client_type_id', 'FROM person TO client_type', '', 'LOOKUP', '', 
    '{"label_en": "Client Type", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}', 
    true, '2da35b3e-1f94-46e6-aa6a-f5ead4d7bb37'
),
(   
    '84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', true, 
    'role_id', 'FROM person TO role', '', 'LOOKUP', '', 
    '{"label_en": "Role", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}', 
    true, 'a94917b8-782d-4355-aec2-d65d7efe2630'
)
ON CONFLICT ("id") DO UPDATE 
SET 
    "table_id" = EXCLUDED."table_id",  
    "required" = EXCLUDED."required",  
    "slug" = EXCLUDED."slug",  
    "label" = EXCLUDED."label",  
    "default" = EXCLUDED."default",  
    "type" = EXCLUDED."type",  
    "index" = EXCLUDED."index",  
    "attributes" = EXCLUDED."attributes",  
    "is_system" = EXCLUDED."is_system",  
    "relation_id" = EXCLUDED."relation_id",
    "updated_at" = NOW();


-- Relation Views
INSERT INTO "view" ( "id", "group_fields", "view_fields", "users", "columns", "time_interval",
                     "relation_id", "updated_fields", "attributes", "navigate", "order" ) 
VALUES 
    (
        '8f1fde99-cc81-4bb6-87ff-bb86acaa73ff', '{}', '{"04d0889a-b9ba-4f5c-8473-c8447aab350d"}',
        '{}', '{}', 0, '2b1549b1-5490-41f3-8442-d3e116d847fb', 
        '{}', '{"label_en": "Client Type", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}'::jsonb,
        '{}'::jsonb, 0
    ),
    (
        '88e3001b-b62c-4d6c-9b12-c7f920f6331f', '{}', '{"c12adfef-2991-4c6a-9dff-b4ab8810f0df"}',
        '{}', '{}', 0, '84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a',
        '{}', '{"label_en": "Role", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}'::jsonb,
        '{}'::jsonb, 0
    )
ON CONFLICT (id) DO UPDATE SET
    "id" = EXCLUDED."id",
    "group_fields" = EXCLUDED."group_fields",
    "view_fields" = EXCLUDED."view_fields",
    "users" = EXCLUDED."users",
    "columns" = EXCLUDED."columns",
    "time_interval" = EXCLUDED."time_interval",
    "relation_id" = EXCLUDED."relation_id",
    "updated_fields" = EXCLUDED."updated_fields",
    "attributes" = EXCLUDED."attributes",
    "navigate" = EXCLUDED."navigate",
    "order" = EXCLUDED."order";

--Relation View Permission
DO $$
DECLARE 
    role_record RECORD; 
BEGIN
    FOR role_record IN 
        SELECT guid FROM role 
    LOOP
        INSERT INTO view_permission ("role_id", "view_id", "view", "edit", "delete") 
        VALUES (role_record.guid, '8f1fde99-cc81-4bb6-87ff-bb86acaa73ff', true, true, true),
               (role_record.guid, '88e3001b-b62c-4d6c-9b12-c7f920f6331f', true, true, true)
        ON CONFLICT ON CONSTRAINT unique_view_role
        DO UPDATE SET 
            "view" = EXCLUDED."view", 
            "edit" = EXCLUDED."edit", 
            "delete" = EXCLUDED."delete", 
            updated_at = CURRENT_TIMESTAMP;
    END LOOP;
END $$;


-- Fields
INSERT INTO "field" ( "id", "table_id", "required", "slug", "label", "type", "index", "is_system","attributes" )
VALUES
    (
        '4f7ade49-da8a-4534-b3a4-35f2875609b1', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', 
        FALSE, 'login', 'Login', 'SINGLE_LINE', 'string', TRUE,
        '{"label": "", "label_en": "Login", "defaultValue": "", "number_of_rounds": null}' 
    ),
    (
        '88ef053a-ae80-44a0-aad1-055f4405a3ee', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', 
        FALSE, 'password', 'Password', 'PASSWORD', 'string', TRUE, 
        '{"label": "", "label_en": "Password", "defaultValue": "", "number_of_rounds": null}' 
     ),
     (
        '4342bf9d-24ad-4f74-bb7e-156d3b4e1dfd', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', 
        FALSE, 'user_id_auth', 'User ID Auth', 'SINGLE_LINE', 'string', TRUE, 
        '{"label": "", "label_en": "User ID Auth", "defaultValue": "", "number_of_rounds": null}' 
     )
ON CONFLICT ("id") DO UPDATE 
    SET 
        "table_id" = EXCLUDED.table_id,
        "required" = EXCLUDED.required,
        "slug" = EXCLUDED.slug,
        "label" = EXCLUDED.label,
        "type" = EXCLUDED.type,
        "index" = EXCLUDED.index,
        "attributes" = EXCLUDED.attributes,
        "is_system" = EXCLUDED.is_system,
        "updated_at" = NOW();

-- Function For Create field permissions
DO $$ 
DECLARE 
    role_record RECORD;
BEGIN
    FOR role_record IN 
        SELECT guid FROM role  
    LOOP
        INSERT INTO field_permission (role_id, label, table_slug, field_id, edit_permission, view_permission) 
        VALUES 
            (role_record.guid, 'Login', 'person', '4f7ade49-da8a-4534-b3a4-35f2875609b1', true, true),
            (role_record.guid, 'Password', 'person', '88ef053a-ae80-44a0-aad1-055f4405a3ee', true, true),
            (role_record.guid, 'User ID Auth', 'person', '4342bf9d-24ad-4f74-bb7e-156d3b4e1dfd', true, true),
            (role_record.guid, 'FROM person TO client_type', 'person', '2b1549b1-5490-41f3-8442-d3e116d847fb', true, true),
            (role_record.guid, 'FROM person TO role', 'person', '84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a', true, true)
        ON CONFLICT (field_id, role_id) 
        DO UPDATE SET 
            edit_permission = EXCLUDED.edit_permission, 
            view_permission = EXCLUDED.view_permission;
    END LOOP;
END $$;


-- Alter Fields to the table 
ALTER TABLE "person" ADD COLUMN "client_type_id" uuid REFERENCES client_type(guid);

ALTER TABLE "person" ADD COLUMN "role_id" uuid REFERENCES role(guid);

ALTER TABLE "person" ADD COLUMN "login" character varying;

ALTER TABLE "person" ADD COLUMN "password" character varying;

ALTER TABLE "person" ADD COLUMN "user_id_auth" uuid;

-- For Create View to Person Table
INSERT INTO "view" ( "id", "table_slug", "type", "disable_dates", "time_interval", "multiple_insert", "is_editable", 
                        "attributes", "navigate", "default_editable", "creatable", "order" ) 
VALUES (
    '0db9b1a2-00cd-4ce0-897c-5a71a764639a', 'person', 'TABLE', '{}', 60, FALSE, FALSE, 
    '{"name_ru": "", "percent": {"field_id": null}, "summaries": [], "group_by_columns": [], "chart_of_accounts": [{"chart_of_account": []}]}', 
    '{}', FALSE, FALSE, 1
) ON CONFLICT (id) DO UPDATE SET 
    "id" = EXCLUDED."id", "table_slug" = EXCLUDED."table_slug", "type" = EXCLUDED."type", "disable_dates" = EXCLUDED."disable_dates",
    "time_interval" = EXCLUDED."time_interval", "multiple_insert" = EXCLUDED."multiple_insert", "is_editable" = EXCLUDED."is_editable",
    "attributes" = EXCLUDED."attributes", "navigate" = EXCLUDED."navigate", "default_editable" = EXCLUDED."default_editable", 
    "creatable" = EXCLUDED."creatable", "order" = EXCLUDED."order";


-- For Create View Role Permission 
DO $$
DECLARE 
    role_record RECORD; 
BEGIN
    FOR role_record IN 
        SELECT guid FROM role 
    LOOP
        INSERT INTO view_permission ("role_id", "view_id", "view", "edit", "delete") 
        VALUES (role_record.guid, '0db9b1a2-00cd-4ce0-897c-5a71a764639a', true, true, true)
        ON CONFLICT ON CONSTRAINT unique_view_role
        DO UPDATE SET 
            "view" = EXCLUDED."view", 
            "edit" = EXCLUDED."edit", 
            "delete" = EXCLUDED."delete", 
            updated_at = CURRENT_TIMESTAMP;
    END LOOP;
END $$;


-- For Create Layout to Person Table
INSERT INTO "layout" ("id", "table_id", "order", "label", "icon", "type", "is_default", "is_visible_section", "is_modal", "attributes" ) 
VALUES ( '371f662a-3a44-46be-a9fa-92add32b27aa', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', 0, 'New Layout', '', 'SimpleLayout', 
        true, false, false,  '{"label_en": "New Layout"}' ) 
ON CONFLICT (id) DO UPDATE SET 
    "table_id" = EXCLUDED."table_id",
    "order" = EXCLUDED."order",
    "label" = EXCLUDED."label",
    "icon" = EXCLUDED."icon",
    "type" = EXCLUDED."type",
    "is_default" = EXCLUDED."is_default",
    "is_visible_section" = EXCLUDED."is_visible_section",
    "is_modal" = EXCLUDED."is_modal",
    "attributes" = EXCLUDED."attributes";

-- Tab
INSERT INTO "tab" ( "id", "order", "label", "icon", "type", "layout_id", "table_slug", "attributes" ) 
VALUES ( '7d375dd3-d291-4865-8134-4be46b2ce0a5', 1, '', '', 'section', '371f662a-3a44-46be-a9fa-92add32b27aa', '', '{"label_en": "Info"}' ) 
ON CONFLICT (id) DO UPDATE 
SET 
    "order" = EXCLUDED."order",
    "label" = EXCLUDED."label",
    "icon" = EXCLUDED."icon",
    "type" = EXCLUDED."type",
    "layout_id" = EXCLUDED."layout_id",
    "relation_id" = EXCLUDED."relation_id",
    "table_slug" = EXCLUDED."table_slug",
    "attributes" = EXCLUDED."attributes",
    "updated_at" = NOW();

-- Section
INSERT INTO "section" ("id", "order", "column", "label", "icon", "is_summary_section", "fields", "table_id", "tab_id", "attributes" ) 
VALUES  ( 
                'd3b85d85-6327-47eb-8e95-2986a261f1d7', 0, '', '', '', false, 
                '[{"id": "f54d8076-4972-4067-9a91-c178c02c4273", "attributes": {"fields": {"label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Full Name"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Full Name"}, {"id": "4f7ade49-da8a-4534-b3a4-35f2875609b1"}, {"id": "88ef053a-ae80-44a0-aad1-055f4405a3ee"}]',
                'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', '7d375dd3-d291-4865-8134-4be46b2ce0a5', '{}'
        ),
        (
                '58a0ed30-6753-4fa8-bd9e-0bf26bf0b3d3', 1, '', '', '', false,
                '[{"id": "client_type#2da35b3e-1f94-46e6-aa6a-f5ead4d7bb37", "attributes": {"fields": [{"id": "2b1549b1-5490-41f3-8442-d3e116d847fb", "slug": "client_type_id", "type": "LOOKUP", "label": "FROM person TO client_type", "required": true, "table_id": "c1669d87-332c-41ee-84ac-9fb2ac9efdd5", "is_system": true, "attributes": {"label_en": "Client Type", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}, "is_visible": true, "relation_id": "2da35b3e-1f94-46e6-aa6a-f5ead4d7bb37"}]}, "field_name": "Тип клиентов", "relation_type": "Many2One"}, {"id": "role#a94917b8-782d-4355-aec2-d65d7efe2630", "attributes": {"fields": [{"id": "84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a", "slug": "role_id", "type": "LOOKUP", "label": "FROM person TO role", "required": true, "table_id": "c1669d87-332c-41ee-84ac-9fb2ac9efdd5", "is_system": true, "attributes": {"label_en": "Role", "label_to_en": "Person", "table_editable": false, "enable_multi_language": false}, "is_visible": true, "relation_id": "a94917b8-782d-4355-aec2-d65d7efe2630"}]}, "field_name": "Роли", "relation_type": "Many2One"}]',
                'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', '7d375dd3-d291-4865-8134-4be46b2ce0a5', '{}'
        ),
        (
                'fb586d45-32f8-41e1-a992-4f8633f0d60f', 2, '', '', '', false,
                '[{"id": "eb3deeb7-6d34-4e24-b65a-f03e09efd0cf", "attributes": {"fields": {"label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Phone Number"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Phone Number"}, {"id": "d868638d-35d6-4992-8216-7b2f479f722e", "attributes": {"fields": {"label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Email"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Email"}, {"id": "c5b09b80-528d-4987-9105-a2be539255ee", "attributes": {"fields": {"path": {"kind": "stringValue", "stringValue": "Media"}, "label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Image"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Image"}]',
                'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', '7d375dd3-d291-4865-8134-4be46b2ce0a5', '{}'
        ),
        (
                '232fc519-9f0a-4ed5-a9de-3a3875583ac9', 3, '', '', '', false,
                '[{"id": "b92b9b8c-c138-4ce6-9260-b4452a7f5ae2", "attributes": {"label": "", "fields": {"label": {"kind": "stringValue", "stringValue": ""}, "options": {"listValue": {"values": [{"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nhqhnwx94lr5tvlf"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Male"}, "value": {"kind": "stringValue", "stringValue": "male"}}}}, {"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nhqlmt8ivnl7ijvr"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Female"}, "value": {"kind": "stringValue", "stringValue": "female"}}}}, {"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nibeydyfhz3y2lfk"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Other"}, "value": {"kind": "stringValue", "stringValue": "other"}}}}]}}, "label_en": {"kind": "stringValue", "stringValue": "Gender"}, "is_multiselect": {"kind": "boolValue", "boolValue": false}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}, "options": [{"id": "m5np5bol3vnezk3pl49", "icon": "", "color": "", "label": "Male", "value": "male"}, {"id": "m5np5lvljg533uqogzd", "icon": "", "color": "", "label": "Female", "value": "female"}, {"id": "m5np5rtg76rd50ksfmq", "icon": "", "color": "", "label": "Other", "value": "other"}], "label_en": "Gender", "has_color": false, "defaultValue": "", "is_multiselect": false, "number_of_rounds": null}, "field_name": "Gender"}, {"id": "e5a2a21e-a9e2-4e6d-87e8-57b8dd837d48", "attributes": {"fields": {"label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Date of birth"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Date Of Birth"}]',
                'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', '7d375dd3-d291-4865-8134-4be46b2ce0a5', '{}'
        )
ON CONFLICT (id) DO UPDATE SET 
    "order" = EXCLUDED."order",
    "column" = EXCLUDED."column",
    "label" = EXCLUDED."label",
    "icon" = EXCLUDED."icon",
    "is_summary_section" = EXCLUDED."is_summary_section",
    "fields" = EXCLUDED."fields",
    "table_id" = EXCLUDED."table_id",
    "tab_id" = EXCLUDED."tab_id",
    "attributes" = EXCLUDED."attributes",
    "updated_at" = NOW();


ALTER TABLE "person" DROP CONSTRAINT IF EXISTS person_email_key;