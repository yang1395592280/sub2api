DO $$
DECLARE
    fk RECORD;
BEGIN
    -- Preserve audit history for size-bet tables even when users are hard-deleted.
    FOR fk IN
        SELECT
            n.nspname AS schema_name,
            c.relname AS table_name,
            con.conname AS constraint_name
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_class refc ON refc.oid = con.confrelid
        JOIN pg_namespace refn ON refn.oid = refc.relnamespace
        JOIN pg_attribute a ON a.attrelid = con.conrelid
            AND a.attnum = ANY(con.conkey)
        WHERE con.contype = 'f'
          AND n.nspname = current_schema()
          AND c.relname IN ('game_bets', 'game_wallet_ledger', 'game_rank_snapshots')
          AND a.attname = 'user_id'
          AND refn.nspname = current_schema()
          AND refc.relname = 'users'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I',
            fk.schema_name,
            fk.table_name,
            fk.constraint_name
        );
    END LOOP;
END $$;
