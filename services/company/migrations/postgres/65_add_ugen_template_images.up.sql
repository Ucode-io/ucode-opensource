ALTER TABLE ugen_template
    ADD COLUMN IF NOT EXISTS images TEXT[] DEFAULT '{}';
