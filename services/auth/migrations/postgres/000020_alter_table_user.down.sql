DROP INDEX IF EXISTS  user_unq_login;

DROP INDEX IF EXISTS  user_unq_phone;

DROP INDEX IF EXISTS  user_login_and_phone_and_email;

ALTER TABLE "user" ADD CONSTRAINT user_unq_login UNIQUE (login);

ALTER TABLE "user" ADD CONSTRAINT user_unq_phone UNIQUE (phone);
