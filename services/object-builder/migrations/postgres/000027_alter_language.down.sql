DELETE FROM "language";

ALTER TABLE "language" DROP CONSTRAINT unique_language_key_category_platform;
