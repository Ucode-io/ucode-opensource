DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'project_folders_type' AND typtype = 'e') THEN
        CREATE TYPE project_folders_type AS ENUM ('FOLDER', 'PROJECT', 'CHAT');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS project_folders (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    label          VARCHAR(255) NOT NULL DEFAULT '',
    parent_id      UUID REFERENCES project_folders(id) ON DELETE CASCADE,
    type           project_folders_type NOT NULL,
    icon           TEXT NOT NULL DEFAULT '',
    order_number   INT NOT NULL DEFAULT 0,
    mcp_project_id UUID REFERENCES mcp_project(id) ON DELETE SET NULL,
    chat_id        UUID REFERENCES chats(id) ON DELETE SET NULL,
    attributes     JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);