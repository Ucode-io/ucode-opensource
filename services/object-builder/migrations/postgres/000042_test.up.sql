DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'response_status'
    ) THEN
CREATE TYPE response_status AS ENUM ('success', 'error');
END IF;
END
$$;

CREATE TABLE IF NOT EXISTS function_logs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    function_id      UUID REFERENCES function(id) ON DELETE CASCADE NULL,
    table_slug       VARCHAR(255)     DEFAULT '',
    request_method   VARCHAR(255)     DEFAULT '',
    action_type      VARCHAR(255)     DEFAULT '',
    send_at          TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    completed_at     TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    duration         INT              DEFAULT 0,
    compute          VARCHAR(255)     DEFAULT '',
    status           response_status  NOT NULL,
    db_bandwidth     VARCHAR(255)     DEFAULT '',
    file_bandwidth   VARCHAR(255)     DEFAULT '',
    vector_bandwidth VARCHAR(255)     DEFAULT '',
    return_size      INT              DEFAULT 0,
    created_at       TIMESTAMP        DEFAULT CURRENT_TIMESTAMP
);
