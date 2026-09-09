CREATE TYPE product_type AS ENUM ('ucode', 'ugen');

ALTER TABLE fare
    ADD COLUMN product_type product_type NOT NULL DEFAULT 'ucode'::product_type;
