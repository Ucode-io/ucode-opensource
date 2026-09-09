ALTER TABLE IF EXISTS "field" ADD CONSTRAINT "field_table_id_slug_unique" UNIQUE (table_id, slug);
