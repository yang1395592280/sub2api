ALTER TABLE zenxiang_liyu_settings
    ADD COLUMN IF NOT EXISTS lucky_coin_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS lucky_coin_double_probability NUMERIC(12,8) NOT NULL DEFAULT 50;

ALTER TABLE zenxiang_liyu_settings
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_settings_lucky_probability_range;

ALTER TABLE zenxiang_liyu_settings
    ADD CONSTRAINT zenxiang_liyu_settings_lucky_probability_range
    CHECK (lucky_coin_double_probability >= 0 AND lucky_coin_double_probability <= 100);

ALTER TABLE zenxiang_liyu_records
    ADD COLUMN IF NOT EXISTS lucky_coin_played BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS lucky_coin_outcome VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS lucky_coin_adjustment NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lucky_coin_played_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS balance_after_lucky NUMERIC(20,8) NULL;

ALTER TABLE zenxiang_liyu_records
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_records_lucky_coin_outcome_check;

ALTER TABLE zenxiang_liyu_records
    ADD CONSTRAINT zenxiang_liyu_records_lucky_coin_outcome_check
    CHECK (lucky_coin_outcome IN ('', 'double', 'zero'));
