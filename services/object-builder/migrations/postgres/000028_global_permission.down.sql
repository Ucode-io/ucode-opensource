DELETE FROM field 
WHERE slug IN ('gitbook_button', 'chatwoot_button', 'gpt_button');

ALTER TABLE IF EXISTS "global_permission" 
DROP COLUMN IF EXISTS "chatwoot_button";

ALTER TABLE IF EXISTS "global_permission" 
DROP COLUMN IF EXISTS "gitbook_button";

ALTER TABLE IF EXISTS "global_permission" 
DROP COLUMN IF EXISTS "gpt_button";
