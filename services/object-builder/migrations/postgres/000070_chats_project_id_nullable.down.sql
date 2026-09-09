DO $$
BEGIN
    IF to_regclass('public.chats') IS NOT NULL THEN
        ALTER TABLE chats ALTER COLUMN project_id SET NOT NULL;
    END IF;
END
$$;
