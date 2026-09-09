DO $$
BEGIN
    IF to_regclass('public.messages') IS NOT NULL THEN
        CREATE TABLE IF NOT EXISTS message_reactions
        (
            id            UUID PRIMARY KEY     DEFAULT uuid_generate_v4(),
            message_id    UUID        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
            user_id       TEXT        NOT NULL,
            reaction_type VARCHAR     NOT NULL CHECK (reaction_type IN ('like', 'dislike')),
            created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            deleted_at    BIGINT      NOT NULL DEFAULT 0
        );

        CREATE UNIQUE INDEX IF NOT EXISTS message_reactions_message_user_active_idx
            ON message_reactions (message_id, user_id)
            WHERE deleted_at = 0;

        CREATE INDEX IF NOT EXISTS message_reactions_message_type_active_idx
            ON message_reactions (message_id, reaction_type)
            WHERE deleted_at = 0;
    END IF;
END
$$;
