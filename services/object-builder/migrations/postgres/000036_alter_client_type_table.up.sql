ALTER TABLE IF EXISTS "client_type" ADD COLUMN IF NOT EXISTS "columns" TEXT[] DEFAULT '{}';

INSERT INTO field(
    "id",
    "table_id",
    "slug",
    "label",
    "default",
    "type",
    "is_visible",
    "is_system",
    "is_search") VALUES
(
'4e5c83a1-5a1a-48f0-b41d-c1f5d472bc52',
'ed3bf0d9-40a3-4b79-beb4-52506aa0b5ea',
'columns',
'Columns',
'{}',
'ARRAY',
false,
true,
true
);
