ALTER TABLE IF EXISTS "project"
    ADD COLUMN IF NOT EXISTS "icon_categories" TEXT[] DEFAULT '{}';