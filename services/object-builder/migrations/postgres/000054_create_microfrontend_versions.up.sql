CREATE TABLE IF NOT EXISTS microfrontend_versions (
    guid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    microfrontend_id VARCHAR(255) NOT NULL,
    commit_message TEXT,
    files TEXT,
    is_current BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_microfrontend_versions_microfrontend_id ON microfrontend_versions(microfrontend_id);
