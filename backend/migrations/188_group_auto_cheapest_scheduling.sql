ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS allow_auto_cheapest_scheduling BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN groups.allow_auto_cheapest_scheduling IS '是否允许 OpenAI 自动最优惠分组调度到此分组';
