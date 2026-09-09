DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'message_role' AND typtype = 'e') THEN
        CREATE TYPE message_role AS ENUM ('user', 'assistant');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS chats
(
    id           UUID PRIMARY KEY      DEFAULT uuid_generate_v4(),
    project_id   UUID         NOT NULL UNIQUE REFERENCES mcp_project (id) ON DELETE CASCADE,
    title        VARCHAR(100) NOT NULL,
    description  TEXT,
    model        VARCHAR(100) NOT NULL DEFAULT 'claude-sonnet-4-5',
    total_tokens INT          NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages
(
    id          UUID PRIMARY KEY      DEFAULT uuid_generate_v4(),
    chat_id     UUID         NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    role        message_role NOT NULL,
    content     TEXT         NOT NULL,
    images      TEXT[]       NOT NULL DEFAULT '{}',
    has_files   BOOLEAN      NOT NULL DEFAULT FALSE,
    tokens_used INT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS file_versions
(
    id             UUID PRIMARY KEY     DEFAULT uuid_generate_v4(),
    file_id        UUID        NOT NULL REFERENCES project_files (id) ON DELETE CASCADE,
    message_id     UUID        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    version_num    INT         NOT NULL,
    content        TEXT        NOT NULL DEFAULT '',
    file_graph     JSONB       NOT NULL DEFAULT '{}',
    change_summary TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (file_id, version_num)
);
