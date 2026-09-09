ALTER TABLE IF EXISTS "view"
    ADD COLUMN IF NOT EXISTS "is_relation_view" BOOLEAN DEFAULT FALSE;

UPDATE view
SET is_relation_view = CASE
    WHEN relation_id IS NULL OR relation_id = '' THEN FALSE
    ELSE TRUE
END;

