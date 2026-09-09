DROP TYPE IF EXISTS service_type;

DROP TABLE IF EXISTS "service_resource";

ALTER TABLE service_resource DROP CONSTRAINT unique_project_id_service_type;