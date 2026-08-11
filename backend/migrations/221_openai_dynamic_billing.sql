-- OpenAI dynamic billing is opt-in per standard group. Existing groups keep
-- their static rate multiplier until explicitly enabled by an administrator.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS dynamic_billing_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.dynamic_billing_enabled IS
    '按账号渠道价格加全局 OpenAI 固定利润动态扣费；仅普通 OpenAI 余额分组可启用';
