ALTER TABLE account_groups
    ADD COLUMN IF NOT EXISTS price_grouping_locked BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN account_groups.price_grouping_locked IS
    '是否保留该账号分组绑定，不受 OpenAI 渠道价格自动归组迁移影响';
