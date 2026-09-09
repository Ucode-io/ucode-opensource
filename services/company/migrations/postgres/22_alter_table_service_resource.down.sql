ALTER TABLE "service_resource" DROP COLUMN IF EXISTS "resource_id";
ALTER TABLE "service_resource" DROP COLUMN IF EXISTS "environment_id";

ALTER TABLE "service_resource" DROP CONSTRAINT unique_project_id_service_type_environment_id;