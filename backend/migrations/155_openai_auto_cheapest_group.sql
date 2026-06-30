ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS group_select_mode VARCHAR(32) NOT NULL DEFAULT 'fixed',
    ADD COLUMN IF NOT EXISTS last_effective_group_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS last_effective_group_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_group_select_mode
    ON api_keys (group_select_mode)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_last_effective_group_id
    ON api_keys (last_effective_group_id)
    WHERE deleted_at IS NULL AND last_effective_group_id IS NOT NULL;
