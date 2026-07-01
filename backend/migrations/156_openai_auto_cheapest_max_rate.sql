ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS openai_auto_group_max_rate_multiplier DECIMAL(20, 8);

COMMENT ON COLUMN api_keys.openai_auto_group_max_rate_multiplier IS
    'Maximum effective group rate multiplier for OpenAI auto cheapest selection (null/0 = unlimited)';
