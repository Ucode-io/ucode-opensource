DROP TABLE IF EXISTS "person";

DELETE FROM "table" WHERE id = 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5';

DELETE FROM "field" WHERE table_id = 'c1669d87-332c-41ee-84ac-9fb2ac9efdd5';

DELETE FROM "record_permission" WHERE table_slug = 'person';

DELETE FROM "field_permission" WHERE table_slug = 'person';