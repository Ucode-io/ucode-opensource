CREATE TABLE IF NOT EXISTS "sms_template" (
    "guid" UUID PRIMARY KEY DEFAULT gen_random_uuid(),            
    "text" TEXT,               
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP
);

INSERT INTO "table" (
    "id", "label", "slug", "description", "deleted_at", "show_in_menu", "is_changed", "icon", 
    "subtitle_field_slug", "folder_id", "is_cached", "soft_delete", "order_by", "is_system"
) 
VALUES (
    'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83', 'SMS Template', 'sms_template', '', NULL, true, false, 
    'comment-sms.svg', '', NULL, false, false, false, true
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
('0a7e1036-b609-4285-984e-95512274fe0a', 'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83', false, 'guid', 'ID', 'v4', 'UUID', 'true', '{}'::jsonb, true, true, true, false, false, NOW(), NOW()),
('6f861c3b-65d0-4217-b1e0-86a9d709443d', 'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83', false, 'text', 'Text', '', 'SINGLE_LINE', 'string', '{"fields": {"label_en": {"stringValue": "Text", "kind": "stringValue"}, "number_of_rounds": {"nullValue": "NULL_VALUE", "kind": "nullValue"}, "defaultValue": {"stringValue": "", "kind": "stringValue"}, "label": {"stringValue": "", "kind": "stringValue"}}}'::jsonb, false, true, false, false, false, NOW(), NOW())
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
            role_record.guid, 'Yes', 'Yes', 'Yes', 'Yes', false, false, 'sms_template', 'Yes',  
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
        ( role_record.guid, 'Text', 'sms_template', '6f861c3b-65d0-4217-b1e0-86a9d709443d', true, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP );
    END LOOP;
END $$;



INSERT INTO "view" ( "id", "table_slug", "type", "disable_dates", "time_interval", "multiple_insert", "is_editable", 
                        "attributes", "navigate", "default_editable", "creatable", "order", "columns" ) 
VALUES (
    '681144a5-ca95-4b34-b702-8030071e2163', 'sms_template', 'TABLE', '{}', 60, FALSE, FALSE, 
    '{"name_ru": "", "percent": {"field_id": null}, "summaries": [], "group_by_columns": [], "chart_of_accounts": [{"chart_of_account": []}]}', 
    '{}', FALSE, FALSE, 1, '{"6f861c3b-65d0-4217-b1e0-86a9d709443d"}'
) ON CONFLICT (id) DO UPDATE SET 
    "id" = EXCLUDED."id", "table_slug" = EXCLUDED."table_slug", "type" = EXCLUDED."type", "disable_dates" = EXCLUDED."disable_dates",
    "time_interval" = EXCLUDED."time_interval", "multiple_insert" = EXCLUDED."multiple_insert", "is_editable" = EXCLUDED."is_editable",
    "attributes" = EXCLUDED."attributes", "navigate" = EXCLUDED."navigate", "default_editable" = EXCLUDED."default_editable", 
    "creatable" = EXCLUDED."creatable", "order" = EXCLUDED."order";



DO $$
DECLARE 
    role_record RECORD; 
BEGIN
    FOR role_record IN 
        SELECT guid FROM role 
    LOOP
        INSERT INTO view_permission ("role_id", "view_id", "view", "edit", "delete") 
        VALUES (role_record.guid, '681144a5-ca95-4b34-b702-8030071e2163', true, true, true)
        ON CONFLICT ON CONSTRAINT unique_view_role
        DO UPDATE SET 
            "view" = EXCLUDED."view", 
            "edit" = EXCLUDED."edit", 
            "delete" = EXCLUDED."delete", 
            updated_at = CURRENT_TIMESTAMP;
    END LOOP;
END $$;


INSERT INTO "layout" ("id", "table_id", "order", "label", "icon", "type", "is_default", "is_visible_section", "is_modal", "attributes" ) 
VALUES ( 'fe0ad42f-d3f0-4268-9dad-d1d6b35fe051', 'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83', 0, 'New Layout', '', 'SimpleLayout', 
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


INSERT INTO "tab" ( "id", "order", "label", "icon", "type", "layout_id", "table_slug", "attributes" ) 
VALUES ( '01a7c6d6-dd92-4f97-ad4b-c10590b5fbe4', 1, '', '', 'section', 'fe0ad42f-d3f0-4268-9dad-d1d6b35fe051', '', '{"label_en": "Info"}' ) 
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


INSERT INTO "section" ("id", "order", "column", "label", "icon", "is_summary_section", "fields", "table_id", "tab_id", "attributes" ) 
VALUES  ( 
                '8d484835-cbbc-4480-b858-c4e68a66cbce', 0, '', '', '', false, 
                '[{"id": "6f861c3b-65d0-4217-b1e0-86a9d709443d", "attributes": {"fields": {"label": {"kind": "stringValue", "stringValue": ""}, "label_en": {"kind": "stringValue", "stringValue": "Text"}, "defaultValue": {"kind": "stringValue", "stringValue": ""}, "number_of_rounds": {"kind": "nullValue", "nullValue": "NULL_VALUE"}}}, "field_name": "Text"}]',
                'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83', '01a7c6d6-dd92-4f97-ad4b-c10590b5fbe4', '{}'
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