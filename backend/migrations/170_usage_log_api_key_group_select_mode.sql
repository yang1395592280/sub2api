ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS api_key_group_select_mode VARCHAR(32) NULL;

COMMENT ON COLUMN usage_logs.api_key_group_select_mode IS
    'API Key group_select_mode snapshot at usage write time. NULL means legacy/unknown.';
