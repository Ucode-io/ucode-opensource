ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_unq_email;

CREATE UNIQUE INDEX IF NOT EXISTS user_unq_email ON "user" (email)
WHERE(
   NOT (
     ( email IS NULL  OR  email = '' )
   )
);