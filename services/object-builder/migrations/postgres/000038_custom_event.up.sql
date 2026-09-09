ALTER TABLE IF EXISTS "custom_event" ADD COLUMN IF NOT EXISTS "path" VARCHAR(255) NOT NULL DEFAULT '';

INSERT INTO "function" (
    "id",
    "name",
    "path",
    "type",
    "description"
) VALUES ('b90d8ad8-553a-4494-8031-660b85a79b45', 'N8N WORKFLOW', '', 'WORKFLOW', 'n8n webhook function');
