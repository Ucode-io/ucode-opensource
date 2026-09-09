CREATE TABLE IF NOT EXISTS ugen_template_reaction (
    id               VARCHAR PRIMARY KEY,
    ugen_template_id VARCHAR NOT NULL REFERENCES ugen_template(id) ON DELETE CASCADE,
    user_id          VARCHAR NOT NULL,
    reaction_type    VARCHAR NOT NULL CHECK (reaction_type IN ('like', 'dislike')),
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at       BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS ugen_template_reaction_active_user_idx
    ON ugen_template_reaction (ugen_template_id, user_id)
    WHERE deleted_at = 0;

CREATE INDEX IF NOT EXISTS ugen_template_reaction_template_type_idx
    ON ugen_template_reaction (ugen_template_id, reaction_type)
    WHERE deleted_at = 0;
