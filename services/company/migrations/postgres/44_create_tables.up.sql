CREATE TYPE project_status AS ENUM ('active', 'inactive', 'blocked');

CREATE TABLE IF NOT EXISTS monthly_requests (
  project_id        UUID REFERENCES project(id) NOT NULL,
  month             INT NOT NULL,
  year              INT NOT NULL,
  requests          INT NOT NULL,
  UNIQUE(project_id, month, year)
);

ALTER TABLE IF EXISTS fare_item_price
    ADD COLUMN IF NOT EXISTS item_type fare_item_type NOT NULL;

ALTER TABLE IF EXISTS project
    ADD COLUMN IF NOT EXISTS status project_status NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS new_design BOOLEAN DEFAULT TRUE;

CREATE UNIQUE INDEX unique_fare_item_type_idx ON fare_item_price (fare_id, item_type);
UPDATE project SET new_design = FALSE;

DROP TABLE IF EXISTS "integration_resource";
DROP TABLE IF EXISTS "project_login_microfront";