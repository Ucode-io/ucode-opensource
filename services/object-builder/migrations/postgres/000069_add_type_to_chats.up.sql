DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'chat_type' AND typtype = 'e') THEN
        CREATE TYPE chat_type AS ENUM ('ucode', 'ugen');
    END IF;
END
$$;

ALTER TABLE chats
    ADD COLUMN IF NOT EXISTS type chat_type NOT NULL DEFAULT 'ucode';