-- Delete from section
DELETE FROM "section" WHERE "id" = '8d484835-cbbc-4480-b858-c4e68a66cbce';

-- Delete from tab
DELETE FROM "tab" WHERE "id" = '01a7c6d6-dd92-4f97-ad4b-c10590b5fbe4';

-- Delete from layout
DELETE FROM "layout" WHERE "id" = 'fe0ad42f-d3f0-4268-9dad-d1d6b35fe051';

-- Delete from view_permission
DELETE FROM "view_permission" WHERE "view_id" = '681144a5-ca95-4b34-b702-8030071e2163';

-- Delete from view
DELETE FROM "view" WHERE "id" = '681144a5-ca95-4b34-b702-8030071e2163';

-- Delete from field_permission
DELETE FROM "field_permission" 
WHERE "table_slug" = 'sms_template' 
AND "field_id" = '6f861c3b-65d0-4217-b1e0-86a9d709443d';

-- Delete from record_permission
DELETE FROM "record_permission" WHERE "table_slug" = 'sms_template';

-- Delete from field
DELETE FROM "field" WHERE "table_id" = 'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83';

-- Delete from table
DELETE FROM "table" WHERE "id" = 'c5ef7f8f-f76b-4cb8-afd9-387f45d88a83';

-- Drop table sms_template
DROP TABLE IF EXISTS "sms_template";
