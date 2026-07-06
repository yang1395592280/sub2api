ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_balance_refresh_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_balance_refresh_interval_seconds INTEGER NOT NULL DEFAULT 600;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_price_max_multiplier DECIMAL(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN groups.upstream_balance_refresh_enabled IS '是否启用分组级上游余额定时刷新';
COMMENT ON COLUMN groups.upstream_balance_refresh_interval_seconds IS '分组级上游余额刷新间隔秒数';
COMMENT ON COLUMN groups.upstream_price_max_multiplier IS '分组级上游价格倍率上限，0 表示不限制';
