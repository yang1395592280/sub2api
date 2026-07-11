ALTER TABLE zenxiang_liyu_settings
    ADD COLUMN IF NOT EXISTS ticket_usage_threshold NUMERIC(20,8) NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS daily_ticket_limit INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS unit_sale_price NUMERIC(20,8) NOT NULL DEFAULT 0.1,
    ADD COLUMN IF NOT EXISTS unit_cost_price NUMERIC(20,8) NOT NULL DEFAULT 0.05;

ALTER TABLE zenxiang_liyu_settings
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_settings_positive_ticket_threshold,
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_settings_positive_daily_ticket_limit,
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_settings_non_negative_unit_sale,
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_settings_non_negative_unit_cost;

ALTER TABLE zenxiang_liyu_settings
    ADD CONSTRAINT zenxiang_liyu_settings_positive_ticket_threshold CHECK (ticket_usage_threshold > 0),
    ADD CONSTRAINT zenxiang_liyu_settings_positive_daily_ticket_limit CHECK (daily_ticket_limit > 0),
    ADD CONSTRAINT zenxiang_liyu_settings_non_negative_unit_sale CHECK (unit_sale_price >= 0),
    ADD CONSTRAINT zenxiang_liyu_settings_non_negative_unit_cost CHECK (unit_cost_price >= 0);

ALTER TABLE zenxiang_liyu_records
    DROP CONSTRAINT IF EXISTS zenxiang_liyu_records_non_negative_ticket;

ALTER TABLE zenxiang_liyu_records
    ADD CONSTRAINT zenxiang_liyu_records_non_negative_ticket CHECK (ticket_amount >= 0);
