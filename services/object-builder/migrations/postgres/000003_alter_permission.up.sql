ALTER TABLE menu_permission
ADD CONSTRAINT unique_menu_role UNIQUE (menu_id, role_id);

ALTER TABLE record_permission
ADD CONSTRAINT unique_role_table_slug UNIQUE (role_id, table_slug);

ALTER TABLE global_permission
ADD CONSTRAINT unique_role_id UNIQUE (role_id);

ALTER TABLE field_permission
ADD CONSTRAINT unique_field_role UNIQUE (field_id, role_id);

ALTER TABLE view_permission
ADD CONSTRAINT unique_view_role UNIQUE (view_id, role_id); 
