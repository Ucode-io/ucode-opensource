-- Intentionally a no-op.
--
-- This is a repair migration: the columns it ensures are required by the running
-- code and legitimately exist on healthy tenants. Dropping them on `down` would be
-- destructive and would re-break the version-history listing, so we do not reverse it.
-- (Columns are still individually reversible via 000051/000052/000055 down migrations.)
SELECT 1;
