ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_unq_login;

ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_unq_phone;

CREATE UNIQUE INDEX IF NOT EXISTS user_unq_login ON "user" (login)
WHERE(
   NOT (
     ( login IS NULL  OR  login = '' )
   )
);

CREATE UNIQUE INDEX IF NOT EXISTS user_unq_phone ON "user" (phone)
WHERE(
   NOT (
     ( phone IS NULL  OR  phone = '' )
   )
);

CREATE UNIQUE INDEX IF NOT EXISTS user_login_and_phone_and_email ON "user" (phone, login, email)
WHERE (
   NOT (
     ( login IS NULL  OR  login = '' )
     AND
     ( phone IS NULL  OR  phone = '' )
   )
);