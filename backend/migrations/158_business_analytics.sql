ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS channel_price_snapshot DECIMAL(12,6),
    ADD COLUMN IF NOT EXISTS channel_price_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS channel_price_refreshed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_usage_logs_channel_price_refreshed_at
    ON usage_logs (channel_price_refreshed_at)
    WHERE channel_price_refreshed_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS business_usage_daily (
    bucket_date DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    channel_id BIGINT NOT NULL DEFAULT 0,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    requests BIGINT NOT NULL DEFAULT 0,
    active_users BIGINT NOT NULL DEFAULT 0,
    active_api_keys BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    avg_group_rate_multiplier NUMERIC(10,4),
    avg_channel_price NUMERIC(12,6),
    missing_channel_price_records BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_date, group_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_business_usage_daily_group_date
    ON business_usage_daily (group_id, bucket_date DESC);
CREATE INDEX IF NOT EXISTS idx_business_usage_daily_account_date
    ON business_usage_daily (account_id, bucket_date DESC);

CREATE TABLE IF NOT EXISTS business_usage_weekly (
    week_start DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    channel_id BIGINT NOT NULL DEFAULT 0,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    requests BIGINT NOT NULL DEFAULT 0,
    active_users BIGINT NOT NULL DEFAULT 0,
    active_api_keys BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    avg_group_rate_multiplier NUMERIC(10,4),
    avg_channel_price NUMERIC(12,6),
    missing_channel_price_records BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (week_start, group_id, account_id)
);

CREATE TABLE IF NOT EXISTS business_usage_daily_users (
    bucket_date DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL,
    requests BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_date, group_id, account_id, user_id)
);
