DO $$
BEGIN
    IF to_regclass('public.chats') IS NOT NULL THEN
        UPDATE chats SET type = 'ucode';
    END IF;
END
$$;
