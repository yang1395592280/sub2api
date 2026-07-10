DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'zenxiang_liyu_settings_singleton'
          AND conrelid = 'zenxiang_liyu_settings'::regclass
    ) THEN
        ALTER TABLE zenxiang_liyu_settings
            ADD CONSTRAINT zenxiang_liyu_settings_singleton CHECK (id = 1);
    END IF;
END $$;

SELECT setval(
    pg_get_serial_sequence('zenxiang_liyu_settings', 'id'),
    GREATEST(COALESCE((SELECT MAX(id) FROM zenxiang_liyu_settings), 1), 1),
    TRUE
);

CREATE OR REPLACE FUNCTION zenxiang_liyu_records_prevent_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'zenxiang_liyu_records are immutable';
END;
$$;

DROP TRIGGER IF EXISTS zenxiang_liyu_records_prevent_mutation
    ON zenxiang_liyu_records;

CREATE TRIGGER zenxiang_liyu_records_prevent_mutation
    BEFORE UPDATE OR DELETE ON zenxiang_liyu_records
    FOR EACH ROW
    EXECUTE FUNCTION zenxiang_liyu_records_prevent_mutation();
