INSERT INTO "project"("id", "name", "domain") VALUES ('f5955c82-f264-4655-aeb4-86fd1c642cb6', 'MEDION', 'medion.udevs.io');

INSERT INTO "client_platform"("id", "project_id", "name", "subdomain") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'MEDION ADMIN PANEL', 'admin.medion.uz');

INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c01', 'ADMIN', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'UNDECIDED', FALSE, FALSE);
INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c02', 'CASHIER', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'UNDECIDED', FALSE, FALSE);
INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'RECORDER', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'UNDECIDED', FALSE, FALSE);
INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c04', 'DOCTOR', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'PHONE', TRUE, TRUE);
INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c05', 'PATIENT', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'PHONE', TRUE, TRUE);
INSERT INTO "client_type"("id", "name", "project_id", "confirm_by", "self_register", "self_recover") VALUES ('5a3818a9-90f0-44e9-a053-3be0ba1e2c06', 'INSURER', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'EMAIL', FALSE, TRUE);


INSERT INTO "relation"("id", "client_type_id", "type", "name", "description") VALUES ('2d4a4c38-90f0-44e9-b744-7be0ba1e2c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'BRANCH', 'MEDION CLINIC', 'Медицинский центр MEDION CLINIC, AESTHETIC & SPA');
INSERT INTO "relation"("id", "client_type_id", "type", "name", "description") VALUES ('2d4a4c38-90f0-44e9-b744-7be0ba1e2c02', '5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'BRANCH', 'MEDION INNOVATION', 'Медицинский центр MEDION INNOVATION');
INSERT INTO "relation"("id", "client_type_id", "type", "name", "description") VALUES ('2d4a4c38-90f0-44e9-b744-7be0ba1e2c03', '5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'BRANCH', 'MEDION FAMILY HOSPITAL', 'Медицинский центр MEDION FAMILY HOSPITAL');

INSERT INTO "user_info_field"("id", "client_type_id", "field_name", "field_type", "data_type") VALUES ('3a3818a9-90f0-44e9-b744-5be0ba1e2c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c04', 'resume_url', 'FLAT', 'STRING');
INSERT INTO "user_info_field"("id", "client_type_id", "field_name", "field_type", "data_type") VALUES ('3a3818a9-90f0-44e9-b744-5be0ba1e2c02', '5a3818a9-90f0-44e9-a053-3be0ba1e2c04', 'contact_links', 'ARRAY', 'STRING');

INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c01', 'STANDARD', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');
INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c02', 'STANDARD', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');
INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'STANDARD', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');
INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c04', 'OTP', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');
INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c05', 'OTP', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');
INSERT INTO "client"("client_platform_id", "client_type_id", "login_strategy", "project_id") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c06', 'STANDARD', 'f5955c82-f264-4655-aeb4-86fd1c642cb6');


INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde01', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c01', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');
INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde02', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c02', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');
INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde03', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c03', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');
INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde04', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c04', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');
INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde05', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c05', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');
INSERT INTO "role"("id", "client_platform_id", "client_type_id", "project_id", "name") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde06', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c06', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', 'DEFAULT');


INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/', 'GET', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/ping', 'GET', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'POST', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'GET', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table/:table_id', 'GET', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'PUT', 1);
INSERT INTO "scope"("client_platform_id", "path", "method", "requests") VALUES ('7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table/:table_id', 'DELETE', 1);

INSERT INTO "permission"("id", "parent_id", "client_platform_id", "name") VALUES ('ffffffff-ffff-4fff-8fff-ffffffffffff', NULL, '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/root');

INSERT INTO "permission"("id", "parent_id", "client_platform_id", "name") VALUES ('9cbb32da-e473-4312-8413-95524ec08c31', 'ffffffff-ffff-4fff-8fff-ffffffffffff', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/root/settings');
INSERT INTO "permission"("id", "parent_id", "client_platform_id", "name") VALUES ('9cbb32da-e473-4312-8413-95524ec08c32', '9cbb32da-e473-4312-8413-95524ec08c31', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/root/settings/constructor');

INSERT INTO "permission_scope"("permission_id", "client_platform_id", "path", "method") VALUES ('9cbb32da-e473-4312-8413-95524ec08c32', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'GET');
INSERT INTO "permission_scope"("permission_id", "client_platform_id", "path", "method") VALUES ('9cbb32da-e473-4312-8413-95524ec08c32', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'POST');
INSERT INTO "permission_scope"("permission_id", "client_platform_id", "path", "method") VALUES ('9cbb32da-e473-4312-8413-95524ec08c32', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table/:table_id', 'GET');
INSERT INTO "permission_scope"("permission_id", "client_platform_id", "path", "method") VALUES ('9cbb32da-e473-4312-8413-95524ec08c32', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '/v1/table', 'PUT');

INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde01', 'ffffffff-ffff-4fff-8fff-ffffffffffff');
INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde02', 'ffffffff-ffff-4fff-8fff-ffffffffffff');
INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde03', 'ffffffff-ffff-4fff-8fff-ffffffffffff');
INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde04', 'ffffffff-ffff-4fff-8fff-ffffffffffff');
INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde05', 'ffffffff-ffff-4fff-8fff-ffffffffffff');
INSERT INTO "role_permission"("role_id", "permission_id") VALUES ('a1ca1301-4da9-424d-a9e2-578ae6dcde06', 'ffffffff-ffff-4fff-8fff-ffffffffffff');

INSERT INTO "user"("id", "project_id", "client_platform_id", "client_type_id", "role_id", "phone", "email", "login", "password", "active", "expires_at")
    VALUES ('f799f1c5-ce5f-4fdd-ac23-f542247dcc01', 'f5955c82-f264-4655-aeb4-86fd1c642cb6', '7d4a4c38-dd84-4902-b744-0488b80a4c01', '5a3818a9-90f0-44e9-a053-3be0ba1e2c01', 'a1ca1301-4da9-424d-a9e2-578ae6dcde01', '+998914015636', 'bakhodir_tukhtamuradov@udevs.io', 'medion_admin', '$argon2id$v=19$models=65536,t=3,p=4$Uv38ByGCZU8WP18PmmIdcg$pkQBTSchMryxoPqiqY6onQlZ7lPSgX1S/HqnfPDIGzk', 1, '2072-05-01T11:21:59.001+0000');
