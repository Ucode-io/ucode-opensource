-- One-time: create a function that returns a nested JSON tree for a given menu id
CREATE OR REPLACE FUNCTION get_menu_tree(p_id uuid)
RETURNS jsonb
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
  result jsonb;
BEGIN
  SELECT jsonb_build_object(
    'id', m.id,
    'label', m.label,
    'type', m.type,
    'icon', m.icon,
    'is_visible', m.is_visible,
    'is_static', m.is_static,
    'order', m."order",
    'attributes', m.attributes,
    'bucket_path', m.bucket_path,
    'layout_id', m.layout_id,
    'table_id', m.table_id,
    'microfrontend_id', m.microfrontend_id,
    'menu_settings_id', m.menu_settings_id,
    'webpage_id', m.webpage_id,
    'parent_id', m.parent_id,
    'children', COALESCE(
      (
        SELECT jsonb_agg(get_menu_tree(c.id) ORDER BY c."order")
        FROM menu c
        WHERE c.parent_id = m.id
      ),
      '[]'::jsonb
    )
  )
  INTO result
  FROM menu m
  WHERE m.id = p_id;

  RETURN result;
END
$$;
