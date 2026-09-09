DELETE FROM menu
WHERE id IN (
    SELECT menu_id
    FROM view
    WHERE table_slug = 'sms_template'
);

DELETE FROM view
WHERE table_slug = 'sms_template';
