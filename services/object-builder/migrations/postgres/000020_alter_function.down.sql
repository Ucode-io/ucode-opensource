ALTER TABLE IF EXISTS "function"
    ADD CONSTRAINT function_type_check CHECK (type IN ('FUNCTION', 'MICRO_FRONTEND'));
