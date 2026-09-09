ALTER TABLE "language"  
DROP COLUMN IF EXISTS "category",  
DROP COLUMN IF EXISTS "platform";

DROP TYPE IF EXISTS category_type;