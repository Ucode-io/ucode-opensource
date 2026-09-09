CREATE TABLE IF NOT EXISTS billing_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    date_time TIMESTAMP NOT NULL,
    time_range BIGINT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    UNIQUE(project_id, date_time, time_range)
);
