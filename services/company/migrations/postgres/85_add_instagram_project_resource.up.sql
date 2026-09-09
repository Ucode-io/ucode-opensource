ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'INSTAGRAM';

-- One Instagram professional account should not be silently reused by multiple
-- generated projects. Avoid type = 'INSTAGRAM' here because PostgreSQL rejects
-- using a newly-added enum value inside the same transaction.
CREATE UNIQUE INDEX IF NOT EXISTS project_resource_instagram_ig_id_unique
    ON project_resource ((settings #>> '{instagram,ig_id}'))
    WHERE settings ? 'instagram'
      AND COALESCE(settings #>> '{instagram,ig_id}', '') <> '';
