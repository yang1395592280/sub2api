ALTER TABLE zenxiang_liyu_records
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_records_request_unique;

ALTER TABLE zenxiang_liyu_records
    DROP CONSTRAINT IF EXISTS zenxiangliyurecord_request_id;

DROP INDEX IF EXISTS zenxiangliyurecord_request_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'zenxiang_liyu_records_user_request_unique'
          AND conrelid = 'zenxiang_liyu_records'::regclass
    ) THEN
        ALTER TABLE zenxiang_liyu_records
            ADD CONSTRAINT zenxiang_liyu_records_user_request_unique UNIQUE (user_id, request_id);
    END IF;
END $$;
