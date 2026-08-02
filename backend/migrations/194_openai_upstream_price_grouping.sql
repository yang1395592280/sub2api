ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_price_grouping_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS upstream_price_grouping_min DECIMAL(12, 6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_price_grouping_max DECIMAL(12, 6) NOT NULL DEFAULT 0;

COMMENT ON COLUMN groups.upstream_price_grouping_enabled IS '是否在刷新渠道价格后按价格区间自动归入 OpenAI 普通分组';
COMMENT ON COLUMN groups.upstream_price_grouping_min IS 'OpenAI 渠道价格自动归组区间下限（包含）';
COMMENT ON COLUMN groups.upstream_price_grouping_max IS 'OpenAI 渠道价格自动归组区间上限（包含）';
