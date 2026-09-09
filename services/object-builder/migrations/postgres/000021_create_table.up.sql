CREATE TABLE IF NOT EXISTS "person" (
    "guid" UUID PRIMARY KEY DEFAULT gen_random_uuid(),            
    "folder_id" UUID,
    "full_name" VARCHAR(100),  
    "email" VARCHAR(255),         
    "phone_number" VARCHAR(20),          
    "gender" TEXT[],                         
    "image" VARCHAR(255), 
    "date_of_birth" DATE,                
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP
);

INSERT INTO "table" (
    "id", "label", "slug", "description", "deleted_at", "show_in_menu", "is_changed", "icon", 
    "subtitle_field_slug", "folder_id", "is_cached", "soft_delete", "order_by", "is_system"
) 
VALUES (
    'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', 'Person', 'person', '', NULL, true, false, 
    'person-circle-check.svg', '', NULL, false, false, false, true
) ON CONFLICT ("id") DO UPDATE 
SET 
    "label" = EXCLUDED."label",
    "slug" = EXCLUDED."slug",
    "description" = EXCLUDED."description",
    "show_in_menu" = EXCLUDED."show_in_menu",
    "is_changed" = EXCLUDED."is_changed",
    "icon" = EXCLUDED."icon",
    "subtitle_field_slug" = EXCLUDED."subtitle_field_slug",
    "folder_id" = EXCLUDED."folder_id",
    "is_cached" = EXCLUDED."is_cached",
    "order_by" = EXCLUDED."order_by",
    "is_system" = EXCLUDED."is_system";


INSERT INTO "field" (
    id, table_id, required, slug, label, "default", type, "index", attributes, 
    is_visible, is_system, "unique", automatic, enable_multilanguage, created_at, updated_at
) VALUES
('fb852fe5-0255-4d2e-8ceb-bf331ae55fb2', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'guid', 'ID', 'v4', 'UUID', 'true', '{}'::jsonb, true, true, true, false, false, NOW(), NOW()),
('f54d8076-4972-4067-9a91-c178c02c4273', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'full_name', 'Full Name', '', 'SINGLE_LINE', 'string', '{"fields": {"label_en": {"stringValue": "Full Name", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}, "defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW()),
('d868638d-35d6-4992-8216-7b2f479f722e', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'email', 'Email', '', 'EMAIL', 'string', '{"fields": {"label_en": {"stringValue": "Email", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}, "defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW()),
('eb3deeb7-6d34-4e24-b65a-f03e09efd0cf', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'phone_number', 'Phone Number', '', 'INTERNATION_PHONE', 'string', '{"fields": {"label_en": {"stringValue": "Phone Number", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}, "defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW()),
('b92b9b8c-c138-4ce6-9260-b4452a7f5ae2', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'gender', 'Gender', '', 'MULTISELECT', 'string', '{"label": "", "fields": {"label": {"kind": "stringValue", "stringValue": ""}, "options": {"listValue": {"values": [{"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nhqhnwx94lr5tvlf"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Male"}, "value": {"kind": "stringValue", "stringValue": "male"}}}}, {"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nhqlmt8ivnl7ijvr"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Female"}, "value": {"kind": "stringValue", "stringValue": "female"}}}}, {"structValue": {"fields": {"id": {"kind": "stringValue", "stringValue": "m5nibeydyfhz3y2lfk"}, "icon": {"kind": "stringValue", "stringValue": ""}, "color": {"kind": "stringValue", "stringValue": ""}, "label": {"kind": "stringValue", "stringValue": "Other"}, "value": {"kind": "stringValue", "stringValue": "other"}}}}]}}, "label_en": {"kind": "stringValue", "stringValue": "Gender"}, "is_multiselect": {"kind": "boolValue", "boolValue": false}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}, "options": [{"id": "m5np5bol3vnezk3pl49", "icon": "", "color": "", "label": "Male", "value": "male"}, {"id": "m5np5lvljg533uqogzd", "icon": "", "color": "", "label": "Female", "value": "female"}, {"id": "m5np5rtg76rd50ksfmq", "icon": "", "color": "", "label": "Other", "value": "other"}], "label_en": "Gender", "has_color": false, "defaultValue": "", "is_multiselect": false, "number_of_rounds": null}'::jsonb, false, true, false, false, false, NOW(), NOW()),
('c5b09b80-528d-4987-9105-a2be539255ee', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'image', 'Image', '', 'PHOTO', 'string', '{"fields": {"defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}, "label_en": {"stringValue": "Image", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}, "path": {"stringValue": "Media", "kind": "stringValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW()),
('e5a2a21e-a9e2-4e6d-87e8-57b8dd837d48', 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5', false, 'date_of_birth', 'Date Of Birth', '', 'DATE', 'string', '{"fields": {"defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}, "label_en": {"stringValue": "Date of birth", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    table_id = EXCLUDED.table_id,
    required = EXCLUDED.required,
    slug = EXCLUDED.slug,
    label = EXCLUDED.label,
    "default" = EXCLUDED."default",
    type = EXCLUDED.type,
    "index" = EXCLUDED."index",
    attributes = EXCLUDED.attributes,
    is_visible = EXCLUDED.is_visible,
    is_system = EXCLUDED.is_system,
    "unique" = EXCLUDED."unique",
    automatic = EXCLUDED.automatic,
    enable_multilanguage = EXCLUDED.enable_multilanguage,
    updated_at = EXCLUDED.updated_at;


DO $$ 
DECLARE 
    role_record RECORD; 
BEGIN
    FOR role_record IN 
        SELECT guid FROM role 
    LOOP
        INSERT INTO record_permission (
            role_id, read, write, update, delete, is_public, is_have_condition, table_slug, automation,
            language_btn, settings, share_modal, view_create, add_field, pdf_action, created_at, updated_at, add_filter, field_filter,
            fix_column, columns, "group", excel_menu, tab_group, search_button, deleted_at
        ) 
        VALUES (
            role_record.guid, 'Yes', 'Yes', 'Yes', 'Yes', false, false, 'person', 'Yes',  
            'Yes', 'Yes', 'Yes', 'Yes', 'Yes', 'Yes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,  
            'Yes', 'Yes', 'Yes', 'Yes', 'Yes', 'Yes', 'Yes', 'Yes', NULL 
        );
    END LOOP;
END $$;


DO $$ 
DECLARE 
    role_record RECORD;
BEGIN
    FOR role_record IN 
        SELECT guid FROM role  
    LOOP
        INSERT INTO field_permission ( role_id, label, table_slug, field_id, edit_permission, view_permission, created_at, updated_at ) 
        VALUES
        ( role_record.guid, 'Full Name', 'person', 'f54d8076-4972-4067-9a91-c178c02c4273', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP ),
        ( role_record.guid, 'Email', 'person', 'd868638d-35d6-4992-8216-7b2f479f722e', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP ),
        ( role_record.guid, 'Phone Number', 'person', 'eb3deeb7-6d34-4e24-b65a-f03e09efd0cf', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP ),
        ( role_record.guid, 'Gender', 'person', 'b92b9b8c-c138-4ce6-9260-b4452a7f5ae2', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP ),
        ( role_record.guid, 'Image', 'person', 'c5b09b80-528d-4987-9105-a2be539255ee', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP ),
        ( role_record.guid, 'Date Of Birth', 'person', 'e5a2a21e-a9e2-4e6d-87e8-57b8dd837d48', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP );
    END LOOP;
END $$;
