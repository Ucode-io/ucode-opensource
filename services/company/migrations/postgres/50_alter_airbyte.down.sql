ALTER TABLE IF EXISTS airbyte 
DROP COLUMN IF EXISTS superset_username,
DROP COLUMN IF EXISTS superset_password;
