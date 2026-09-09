DROP INDEX IF EXISTS idx_project_user_seat_billing_period_status;
DROP TABLE IF EXISTS project_user_seat_billing_period;

-- transaction_type enum values cannot be safely removed from Postgres enums in
-- a reversible migration; leave user_seat_monthly/user_seat_monthly_refund in place.
