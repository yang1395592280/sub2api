ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS auto_grouping_enabled BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN accounts.auto_grouping_enabled IS
    'Whether the account participates in automatic channel-price grouping';
