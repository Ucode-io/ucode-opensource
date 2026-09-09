CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'file_storage_type') THEN
        CREATE TYPE file_storage_type AS ENUM ('minio', 'google_drive');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS "client_platform" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" UUID,
    "name" VARCHAR(255),
    "subdomain" VARCHAR(255),
    "client_type_ids" UUID[],
    "is_system" BOOLEAN DEFAULT true,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "client_type" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" UUID,
    "name" VARCHAR(255),
    "self_register" BOOLEAN DEFAULT false,
    "self_recover" BOOLEAN DEFAULT false,
    "client_platform_ids" UUID[],
    "confirm_by" VARCHAR(255),
    "is_system" BOOLEAN DEFAULT true,
    "default_page" VARCHAR(255),
    "table_slug" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "table" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "label" VARCHAR(255) NOT NULL,
    "slug" VARCHAR(255) NOT NULL UNIQUE,
    "icon" TEXT,
    "description" TEXT,
    "folder_id" UUID,
    "show_in_menu" BOOLEAN DEFAULT true,
    "subtitle_field_slug" VARCHAR(255) DEFAULT '',
    "is_changed" BOOLEAN DEFAULT true,
    "is_system" BOOLEAN DEFAULT false,
    "soft_delete" BOOLEAN DEFAULT false,
    "is_cached" BOOLEAN DEFAULT false,
    "digit_number" SMALLINT DEFAULT 0,
    "is_changed_by_host" JSONB DEFAULT '{}',
    "commit_guid" UUID,
    "is_login_table" BOOLEAN DEFAULT false,
    "attributes" JSONB DEFAULT '{}',
    "order_by" BOOLEAN DEFAULT false,
    "section_column_count" INTEGER DEFAULT 3,
    "with_increment_id" BOOLEAN DEFAULT false,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP 
);

CREATE TABLE IF NOT EXISTS "field" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_id" UUID REFERENCES "table"("id") ON DELETE CASCADE,
    "required" BOOLEAN DEFAULT false,
    "slug" VARCHAR(255) NOT NULL,
    "label" TEXT NOT NULL,
    "default" VARCHAR(255),
    "type" VARCHAR(255),
    "index" VARCHAR(255),
    "attributes" JSONB DEFAULT '{}',
    "is_visible" BOOLEAN DEFAULT true,
    "is_system" BOOLEAN DEFAULT false,
    "is_search" BOOLEAN DEFAULT false,
    "autofill_field" VARCHAR(512) DEFAULT '',
    "autofill_table" VARCHAR(512) DEFAULT '',
    "relation_id" UUID,
    "unique" BOOLEAN DEFAULT false,
    "automatic" BOOLEAN DEFAULT false,
    "enable_multilanguage" BOOLEAN DEFAULT false,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


DO
$$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'relation_type' AND typtype = 'e'
    ) THEN
        CREATE TYPE "relation_type" AS ENUM (
            'One2One',
            'One2Many',
            'Many2One',
            'Many2Many',
            'Recursive',
            'Many2Dynamic'
        );
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS "relation" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_from" VARCHAR(255),
    "table_to" VARCHAR(255),
    "field_from" VARCHAR(255),
    "field_to" VARCHAR(255),
    "type" relation_type NOT NULL,
    "view_fields" TEXT[],
    "relation_field_slug" VARCHAR(255),
    "editable" BOOLEAN DEFAULT false,
    "is_user_id_default" BOOLEAN DEFAULT false,
    "cascadings" JSONB DEFAULT '{}',
    "is_system" BOOLEAN DEFAULT true,
    "object_id_from_jwt" BOOLEAN DEFAULT false,
    "cascading_tree_table_slug" VARCHAR(512),
    "cascading_tree_field_slug" VARCHAR(255),
    "dynamic_tables" JSONB DEFAULT '{}',
    "auto_filters" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX ON "relation" USING gin ("dynamic_tables");

CREATE TABLE IF NOT EXISTS "custom_event" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_slug" VARCHAR(255) NOT NULL,
    "icon" TEXT,
    "label" VARCHAR(255) NOT NULL,
    "event_path" UUID,
    "url" TEXT,
    "disable" BOOLEAN DEFAULT false,
    "method" VARCHAR(255),
    "action_type" VARCHAR(255),
    "attributes" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "custom_error_message" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255),
    "title" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "role" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255) NOT NULL,
    "project_id" UUID,
    "client_platform_id" UUID REFERENCES "client_platform"("guid"),
    "client_type_id" UUID REFERENCES "client_type"("guid"),
    "is_system" BOOLEAN DEFAULT false,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "automatic_filter" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_slug" VARCHAR(255),
    "custom_field" VARCHAR(255),
    "object_field" VARCHAR(255),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "method" VARCHAR(255),
    "not_use_in_tab" BOOLEAN DEFAULT false,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "user" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" UUID,
    "client_platform_id" UUID REFERENCES "client_platform"("guid"),
    "client_type_id" UUID REFERENCES "client_type"("guid"),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "active" FLOAT,
    "is_system" BOOLEAN DEFAULT true,
    "user_id_auth" UUID,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "layout" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_id" UUID REFERENCES "table"("id") ON DELETE CASCADE,
    "order" SMALLINT,
    "label" VARCHAR(255),
    "icon" VARCHAR(255),
    "type" VARCHAR(255),
    "is_default" BOOLEAN DEFAULT true,
    "is_visible_section" BOOLEAN DEFAULT false,
    "is_modal" BOOLEAN DEFAULT true,
    "menu_id" VARCHAR(255),
    "attributes" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE IF NOT EXISTS "menu_setting" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "icon_style" VARCHAR(255),
    "icon_size" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "menu" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "label" VARCHAR(255) DEFAULT '',
    "parent_id" UUID REFERENCES "menu"("id") ON DELETE CASCADE,
    "layout_id" UUID REFERENCES "layout"("id") ON DELETE CASCADE,
    "table_id" UUID REFERENCES "table"("id") ON DELETE CASCADE,
    "type" VARCHAR(255) DEFAULT '',
    "icon" TEXT DEFAULT '',
    "microfrontend_id" UUID DEFAULT NULL,
    "menu_settings_id" UUID REFERENCES "menu_setting"("id") ON DELETE CASCADE,
    "is_visible" BOOLEAN DEFAULT false,
    "is_static" BOOLEAN DEFAULT false,
    "order" SERIAL,
    "webpage_id" UUID DEFAULT NULL,
    "attributes" JSONB DEFAULT '{}',
    bucket_path VARCHAR DEFAULT '',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP 
); 

CREATE TABLE IF NOT EXISTS "menu_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "menu_id" UUID REFERENCES "menu"("id") ON DELETE CASCADE,
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "write" BOOLEAN DEFAULT true,
    "read" BOOLEAN DEFAULT true,
    "update" BOOLEAN DEFAULT true,
    "delete" BOOLEAN DEFAULT true,
    "menu_settings" BOOLEAN DEFAULT false,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "tab" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "order" INTEGER NOT NULL,
    "label" VARCHAR(255),
    "icon" VARCHAR(255),
    "type" VARCHAR(255) CHECK (type IN ('relation', 'section')),
    "layout_id" UUID REFERENCES "layout"("id") ON DELETE CASCADE,
    "relation_id" UUID REFERENCES "relation"("id") ON DELETE CASCADE,
    "table_slug" VARCHAR(255),
    "attributes" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "section" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "order" SMALLINT,
    "column" VARCHAR(255),
    "label" VARCHAR(255),
    "icon" VARCHAR(255),
    "is_summary_section" BOOLEAN DEFAULT false,
    "fields" JSONB DEFAULT '[]'::jsonb,
    "table_id" UUID REFERENCES "table"("id") ON DELETE CASCADE,
    "tab_id" UUID REFERENCES "tab"("id") ON DELETE CASCADE,
    "attributes" JSONB DEFAULT '{}',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE IF NOT EXISTS "test_login" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "login_strategy" VARCHAR(255),
    "table_slug" VARCHAR(255),
    "login_label" VARCHAR(255),
    "login_view" VARCHAR(255),
    "password_view" VARCHAR(255),
    "password_label" VARCHAR(255),
    "object_id" VARCHAR(255),
    "client_type_id" UUID REFERENCES "client_type"("guid") ON DELETE CASCADE,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "connection" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_slug" VARCHAR(255),
    "view_slug" VARCHAR(255),
    "view_label" VARCHAR(255),
    "name" VARCHAR(255),
    "type" VARCHAR(255),
    "icon" TEXT,
    "main_table_slug" VARCHAR(255),
    "field_slug" VARCHAR(255),
    "client_type_id" UUID REFERENCES "client_type"("guid") ON DELETE CASCADE,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "currency_setting" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "symbol" VARCHAR(10),
    "name" VARCHAR(255),
    "symbol_native" VARCHAR(10),
    "decimal_digits" SMALLINT,
    "rounding" SMALLINT,
    "code" VARCHAR(10),
    "name_plural" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "language_setting" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255),
    "short_name" VARCHAR(10),
    "native_name" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "timezone_setting" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255),
    "text" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "record_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "read" VARCHAR(255) DEFAULT 'Yes',
    "write" VARCHAR(255) DEFAULT 'Yes',
    "update" VARCHAR(255) DEFAULT 'Yes',
    "delete" VARCHAR(255) DEFAULT 'Yes',
    "is_public" BOOLEAN DEFAULT false,
    "is_have_condition" BOOLEAN DEFAULT false,
    "table_slug" VARCHAR(255),
    "automation" VARCHAR(255) DEFAULT 'Yes',
    "language_btn" VARCHAR(255) DEFAULT 'Yes',
    "settings" VARCHAR(255) DEFAULT 'Yes',
    "share_modal" VARCHAR(255) DEFAULT 'Yes',
    "view_create" VARCHAR(255) DEFAULT 'Yes',
    "add_field" VARCHAR(255) DEFAULT 'Yes',
    "pdf_action" VARCHAR(255) DEFAULT 'Yes',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "action_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "label" VARCHAR(255),
    "table_slug" VARCHAR(255) NOT NULL,
    "permission" BOOLEAN DEFAULT true,
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "custom_event_id" UUID REFERENCES "custom_event"("id") ON DELETE CASCADE,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "field_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "label" VARCHAR(255),
    "table_slug" VARCHAR(255),
    "field_id" UUID REFERENCES "field"("id") ON DELETE CASCADE,
    "edit_permission" BOOLEAN DEFAULT true,
    "view_permission" BOOLEAN DEFAULT true,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "global_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "chat" BOOLEAN DEFAULT true,
    "menu_button" BOOLEAN DEFAULT true,
    "settings_button" BOOLEAN DEFAULT true,
    "projects_button" BOOLEAN DEFAULT true,
    "environments_button" BOOLEAN DEFAULT true,
    "api_keys_button" BOOLEAN DEFAULT true,
    "menu_setting_button" BOOLEAN DEFAULT true,
    "redirects_button" BOOLEAN DEFAULT true,
    "profile_settings_button" BOOLEAN DEFAULT true,
    "project_settings_button" BOOLEAN DEFAULT true,
    "project_button" BOOLEAN DEFAULT true,
    "sms_button" BOOLEAN DEFAULT true,
    "version_button" BOOLEAN DEFAULT true,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "view" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "table_slug" VARCHAR(255),
    "type" VARCHAR(255),
    "group_fields" UUID[],
    "view_fields" UUID[],
    "disable_dates" JSONB DEFAULT '{}',
    "quick_filters" JSONB[],
    "users" UUID[],
    "name" VARCHAR(255),
    "columns" UUID[],
    "calendar_from_slug" VARCHAR(255),
    "calendar_to_slug" VARCHAR(255),
    "time_interval" INTEGER,
    "multiple_insert" BOOLEAN DEFAULT false,
    "status_field_slug" VARCHAR(255),
    "is_editable" BOOLEAN DEFAULT false,
    "relation_table_slug" VARCHAR(255),
    "relation_id" VARCHAR(255),
    "summaries" VARCHAR[],
    "updated_fields" UUID[],
    "default_values" VARCHAR[],
    "app_id" UUID,
    "table_label" VARCHAR(255),
    "action_relations" UUID[],
    "default_limit" VARCHAR(255),
    "attributes" JSONB DEFAULT '{}',
    "navigate" JSONB DEFAULT '{}',
    "default_editable" BOOLEAN DEFAULT false,
    "creatable" BOOLEAN DEFAULT false,
    "function_path" UUID,
    "order" INTEGER DEFAULT 0,
    "name_uz" VARCHAR(255),
    "name_en" VARCHAR(255),
    "created_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "view_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "view_id" UUID REFERENCES "view"("id") ON DELETE CASCADE,
    "view" BOOLEAN DEFAULT true,
    "edit" BOOLEAN DEFAULT true,
    "delete" BOOLEAN DEFAULT true,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "view_relation_permission" (
    "guid" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "role_id" UUID REFERENCES "role"("guid") ON DELETE CASCADE,
    "table_slug" VARCHAR(255),
    "relation_id" UUID REFERENCES "relation"("id") ON DELETE CASCADE,
    "view_permission" BOOLEAN DEFAULT true,
    "create_permission" BOOLEAN DEFAULT true,
    "edit_permission" BOOLEAN DEFAULT true,
    "delete_permission" BOOLEAN DEFAULT true,
    "label" VARCHAR,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "version" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255) UNIQUE,
    "is_current" BOOLEAN DEFAULT false,
    "description" VARCHAR(255),
    "version_number" SMALLINT,
    "user_info" VARCHAR(255),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "version_history" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "action_source" VARCHAR(255) NOT NULL,
    "action_type" VARCHAR(255) NOT NULL,
    "previous" JSONB DEFAULT '{}',
    "current" JSONB DEFAULT '{}',
    "date" VARCHAR(255),
    "user_info" VARCHAR(255) NOT NULL,
    "request" JSONB DEFAULT '{}',
    "response" JSONB DEFAULT '{}',
    "api_key" VARCHAR(255),
    "type" VARCHAR(255) DEFAULT 'GLOBAL',
    "table_slug" VARCHAR(255) NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "file" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "title" VARCHAR(255) NOT NULL,
    "description" TEXT,
    "tags" VARCHAR[],
    "storage" VARCHAR(255) NOT NULL,
    "storage_type" file_storage_type NOT NULL DEFAULT 'minio',
    "file_name_disk" VARCHAR(255) NOT NULL,
    "file_name_download" VARCHAR(255) NOT NULL,
    "link" VARCHAR(255) NOT NULL,
    "file_size" INTEGER NOT NULL,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "function_folder" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "type" VARCHAR(20) CHECK (type IN ('FUNCTION', 'MICRO_FRONTEND')),
    "title" VARCHAR(255),
    "project_id" UUID,
    "environment_id" UUID,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "function" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "name" VARCHAR(255),
    "path" VARCHAR(255) UNIQUE,
    "type" VARCHAR(20) CHECK (type IN ('FUNCTION', 'MICRO_FRONTEND')),
    "framework_type" VARCHAR(255),
    "description" VARCHAR(255),
    "project_id" UUID,
    "environment_id" UUID,
    "function_folder_id" UUID,
    "request_time" VARCHAR(255),
    "url" VARCHAR(255),
    "password" VARCHAR(255),
    "ssh_url" VARCHAR(255),
    "gitlab_id" VARCHAR(20),
    "gitlab_group_id" VARCHAR(20),
    "source_url" VARCHAR(255),
    "branch" VARCHAR(255),
    "pipeline_status" TEXT,
    "repo_id" VARCHAR(255),
    "error_message" VARCHAR(255),
    "job_name" VARCHAR(255),
    "resource" VARCHAR(255),
    "provided_name" VARCHAR(255),
    "request_type" VARCHAR(255) DEFAULT 'ASYNC',
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "incrementseqs" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "field_slug" VARCHAR(255),
    "table_slug" VARCHAR(255),
    "increment_by" INTEGER DEFAULT 0,
    "min_value" INTEGER DEFAULT 1,
    "max_value" INTEGER DEFAULT 999999999,
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT increment_by_less_than_max CHECK (increment_by <= max_value)
);
