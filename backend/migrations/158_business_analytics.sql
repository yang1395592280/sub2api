ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS channel_price_snapshot DECIMAL(12,6),
    ADD COLUMN IF NOT EXISTS channel_price_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS channel_price_refreshed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_usage_logs_channel_price_refreshed_at
    ON usage_logs (channel_price_refreshed_at)
    WHERE channel_price_refreshed_at IS NOT NULL;
