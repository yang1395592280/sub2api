DO $$
DECLARE
    user_fk_name NAME;
BEGIN
    SELECT key_usage.constraint_name
    INTO user_fk_name
    FROM information_schema.key_column_usage AS key_usage
    JOIN information_schema.referential_constraints AS referential_constraints
        ON referential_constraints.constraint_schema = key_usage.constraint_schema
        AND referential_constraints.constraint_name = key_usage.constraint_name
    WHERE key_usage.table_schema = current_schema()
      AND key_usage.table_name = 'zenxiang_liyu_records'
      AND key_usage.column_name = 'user_id'
      AND referential_constraints.unique_constraint_schema = current_schema()
      AND referential_constraints.unique_constraint_name IN (
          SELECT table_constraints.constraint_name
          FROM information_schema.table_constraints AS table_constraints
          WHERE table_constraints.table_schema = current_schema()
            AND table_constraints.table_name = 'users'
            AND table_constraints.constraint_type = 'PRIMARY KEY'
      );

    IF user_fk_name IS NOT NULL THEN
        EXECUTE format(
            'ALTER TABLE zenxiang_liyu_records DROP CONSTRAINT %I',
            user_fk_name
        );
    END IF;

    ALTER TABLE zenxiang_liyu_records
        ADD CONSTRAINT zenxiang_liyu_records_users_zenxiang_liyu_records
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;
END $$;

CREATE OR REPLACE FUNCTION zenxiang_liyu_records_prevent_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE'
       AND OLD.prize_id IS NOT NULL
       AND NEW.prize_id IS NULL
       AND pg_trigger_depth() > 1
       AND to_jsonb(NEW) - 'prize_id' = to_jsonb(OLD) - 'prize_id' THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'zenxiang_liyu_records are immutable';
END;
$$;

DROP TRIGGER IF EXISTS zenxiang_liyu_records_prevent_mutation
    ON zenxiang_liyu_records;

CREATE TRIGGER zenxiang_liyu_records_prevent_mutation
    BEFORE UPDATE OR DELETE ON zenxiang_liyu_records
    FOR EACH ROW
    EXECUTE FUNCTION zenxiang_liyu_records_prevent_mutation();
