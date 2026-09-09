CREATE TABLE IF NOT EXISTS "project_card" (
    "id" UUID PRIMARY KEY,
    "pan" VARCHAR(255) NOT NULL,
    "expire" VARCHAR(255) NOT NULL,
    "payme_token" TEXT,
    "verify" BOOLEAN DEFAULT FALSE,
    "project_id" UUID REFERENCES "project"("id"),
    "created_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_pan_project UNIQUE ("pan", "project_id")
);
