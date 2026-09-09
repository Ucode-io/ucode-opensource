ALTER TABLE "service_resource" DROP COLUMN resource_environment_id;

ALTER TABLE "service_resource" ADD COLUMN resource_id UUID NOT NULL REFERENCES "resource"(id);
ALTER TABLE "service_resource" ADD COLUMN environment_id UUID NOT NULL REFERENCES "environment"(id);

ALTER TABLE "service_resource" DROP CONSTRAINT unique_project_id_service_type;
ALTER TABLE "service_resource" ADD CONSTRAINT unique_project_id_service_type_environment_id UNIQUE(project_id, service_type, environment_id);