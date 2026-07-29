UPDATE api_keys
SET openai_auto_group_max_rate_multiplier = 0.2
WHERE deleted_at IS NULL
  AND group_select_mode = 'openai_auto_cheapest'
  AND COALESCE(openai_auto_group_max_rate_multiplier, 0) <= 0;

COMMENT ON COLUMN api_keys.openai_auto_group_max_rate_multiplier IS
    'Maximum effective group rate multiplier for OpenAI auto cheapest selection; auto mode defaults to 0.2';
