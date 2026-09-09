-- PostgreSQL cannot remove a value from an enum type; leaving 'IpakYuli' in
-- payment_type is harmless (no rows reference it after the feature is removed).
SELECT 1;
