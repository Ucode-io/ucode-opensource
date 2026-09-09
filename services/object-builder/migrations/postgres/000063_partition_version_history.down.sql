-- WARNING: down does not restore deleted history. It only converts the partitioned table back
-- to a plain table with the same schema, so the migration tool can keep going on rollback.

DROP TABLE IF EXISTS "version_history" CASCADE;

CREATE TABLE IF NOT EXISTS "version_history" (
    "id"                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "action_source"     VARCHAR(255) NOT NULL,
    "action_type"       VARCHAR(255) NOT NULL,
    "previous"          JSONB        DEFAULT '{}',
    "current"           JSONB        DEFAULT '{}',
    "date"              VARCHAR(255),
    "user_info"         VARCHAR(255) NOT NULL,
    "request"           JSONB        DEFAULT '{}',
    "response"          JSONB        DEFAULT '{}',
    "api_key"           VARCHAR(255),
    "type"              VARCHAR(255) DEFAULT 'GLOBAL',
    "table_slug"        VARCHAR(255) NOT NULL,
    "used_environments" JSONB        DEFAULT '{}',
    "deleted_at"        TIMESTAMP,
    "method_api"        VARCHAR(255),
    "time_started"      VARCHAR(255),
    "time_completed"    VARCHAR(255),
    "duration"          BIGINT,
    "status_code"       BIGINT,
    "table_label"       VARCHAR(255) DEFAULT '',
    "created_at"        TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    "updated_at"        TIMESTAMP    DEFAULT CURRENT_TIMESTAMP
);
