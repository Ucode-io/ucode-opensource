CREATE TYPE hash_type AS ENUM ('argon', 'bcrypt');

ALTER TABLE IF EXISTS "user" ADD COLUMN IF NOT EXISTS "hash_type" hash_type NOT NULL DEFAULT 'argon';