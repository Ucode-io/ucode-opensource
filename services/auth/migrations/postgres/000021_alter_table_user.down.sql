DROP INDEX IF EXISTS  user_unq_email;

ALTER TABLE "user" ADD CONSTRAINT user_unq_email UNIQUE(email);