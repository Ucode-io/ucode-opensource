DO $$
BEGIN
    IF to_regclass('public.chats') IS NOT NULL THEN
        UPDATE chats SET type = 'ugen';
    END IF;
END
$$;
