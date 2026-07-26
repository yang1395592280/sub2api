ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS group_role VARCHAR(32) NOT NULL DEFAULT 'standard';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS self_hosted_pool_group_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_groups_group_role'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT chk_groups_group_role
            CHECK (group_role IN ('standard', 'self_hosted_pool'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_groups_self_hosted_pool'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT fk_groups_self_hosted_pool
            FOREIGN KEY (self_hosted_pool_group_id)
            REFERENCES groups(id)
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_groups_group_role
    ON groups(group_role);

CREATE INDEX IF NOT EXISTS idx_groups_self_hosted_pool_group_id
    ON groups(self_hosted_pool_group_id)
    WHERE deleted_at IS NULL AND self_hosted_pool_group_id IS NOT NULL;

COMMENT ON COLUMN groups.group_role IS
    '分组角色：standard 普通业务分组，self_hosted_pool OpenAI 自建号池';

COMMENT ON COLUMN groups.self_hosted_pool_group_id IS
    '普通 OpenAI 分组优先使用的自建号池 ID；允许多个普通分组引用同一号池';

ALTER TABLE openai_scheduler_decision_audits
    ADD COLUMN IF NOT EXISTS effective_group_id BIGINT,
    ADD COLUMN IF NOT EXISTS account_source_group_id BIGINT,
    ADD COLUMN IF NOT EXISTS account_source_type VARCHAR(32),
    ADD COLUMN IF NOT EXISTS pool_group_id BIGINT,
    ADD COLUMN IF NOT EXISTS pool_fallback_reason VARCHAR(64);
