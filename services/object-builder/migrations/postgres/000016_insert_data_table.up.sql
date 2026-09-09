UPDATE "table"
SET "slug" = 'connections'
WHERE "id" = '0ade55f8-c84d-42b7-867f-6418e1314e28';

ALTER TABLE "connection" 
RENAME TO "connections";
