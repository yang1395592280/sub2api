-- Add the built-in GPT configuration guide to the user menu.
-- The migration is idempotent and preserves existing custom menu entries.

DO $$
DECLARE
    v_raw   text;
    v_items jsonb;
    v_item  jsonb;
BEGIN
    SELECT value INTO v_raw
      FROM settings
     WHERE key = 'custom_menu_items';

    IF COALESCE(TRIM(v_raw), '') = '' OR v_raw = 'null' THEN
        v_items := '[]'::jsonb;
    ELSE
        v_items := v_raw::jsonb;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM jsonb_array_elements(v_items) elem
         WHERE elem ->> 'id' = 'gpt-config-guide'
    ) THEN
        RETURN;
    END IF;

    v_item := jsonb_build_object(
        'id', 'gpt-config-guide',
        'label', 'GPT 一键配置',
        'icon_svg', '',
        'url', 'md:gpt-config-guide',
        'page_slug', 'gpt-config-guide',
        'visibility', 'user',
        'sort_order', 900
    );

    v_items := v_items || jsonb_build_array(v_item);

    INSERT INTO settings (key, value)
    VALUES ('custom_menu_items', v_items::text)
    ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
END $$;
