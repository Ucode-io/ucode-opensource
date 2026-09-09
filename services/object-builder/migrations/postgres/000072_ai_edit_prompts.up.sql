-- Every generated child project has its own database, so prompt_kind alone is
-- the row identity. There can be at most one override for each editable prompt.
-- Revisions come from a database sequence instead of restarting at 1 after a
-- reset. This prevents a stale client revision from matching a newly recreated
-- override (the ABA problem).
CREATE SEQUENCE IF NOT EXISTS ai_edit_prompt_revision_seq;

CREATE TABLE IF NOT EXISTS ai_edit_prompts
(
    prompt_kind       TEXT        PRIMARY KEY,
    content           TEXT        NOT NULL,
    revision          BIGINT      NOT NULL DEFAULT nextval('ai_edit_prompt_revision_seq'),
    updated_by_user_id TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ai_edit_prompts_kind_check
        CHECK (prompt_kind IN ('code_editor', 'visual_editor')),
    CONSTRAINT ai_edit_prompts_content_check
        CHECK (content ~ '[^[:space:]]' AND OCTET_LENGTH(content) <= 131072),
    CONSTRAINT ai_edit_prompts_revision_check
        CHECK (revision > 0)
);

-- Keep reruns safe if the table was created by an earlier draft of this
-- migration before sequence-backed revisions were introduced.
ALTER TABLE ai_edit_prompts
    ALTER COLUMN revision SET DEFAULT nextval('ai_edit_prompt_revision_seq');

DO $$
DECLARE
    max_revision BIGINT;
BEGIN
    SELECT MAX(revision) INTO max_revision FROM ai_edit_prompts;
    IF max_revision IS NOT NULL THEN
        PERFORM setval(
            'ai_edit_prompt_revision_seq',
            GREATEST(max_revision, (SELECT last_value FROM ai_edit_prompt_revision_seq)),
            TRUE
        );
    END IF;
END
$$;

ALTER SEQUENCE ai_edit_prompt_revision_seq OWNED BY ai_edit_prompts.revision;
