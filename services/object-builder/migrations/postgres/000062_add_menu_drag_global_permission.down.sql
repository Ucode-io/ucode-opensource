DELETE FROM field
WHERE table_id = '1b066143-9aad-4b28-bd34-0032709e463b'
  AND slug = 'menu_drag';

ALTER TABLE IF EXISTS "global_permission"
    DROP COLUMN IF EXISTS "menu_drag";
