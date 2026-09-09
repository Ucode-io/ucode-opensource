DROP TABLE IF EXISTS redirect_url;

CREATE TABLE IF NOT EXISTS redirect_url(
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES project(id),
    env_id UUID REFERENCES environment(id),
    "from" VARCHAR,
    "to" VARCHAR,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);