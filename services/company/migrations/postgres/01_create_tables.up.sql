CREATE TABLE IF NOT EXISTS company (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    logo VARCHAR NOT NULL,
    description TEXT,
    updated_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
 );

 CREATE TABLE IF NOT EXISTS project (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    company_id uuid REFERENCES company (id),
    k8s_namespace VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
 );
