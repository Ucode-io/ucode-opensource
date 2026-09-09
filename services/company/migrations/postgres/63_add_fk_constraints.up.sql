ALTER TABLE billing_usage
    ADD CONSTRAINT fk_billing_usage_project_id FOREIGN KEY (project_id) REFERENCES project(id);

ALTER TABLE integration_resource
    ADD CONSTRAINT fk_integration_resource_project_id FOREIGN KEY (project_id) REFERENCES project(id);

ALTER TABLE integration_resource
    ADD CONSTRAINT fk_integration_resource_environment_id FOREIGN KEY (environment_id) REFERENCES "environment"(id);