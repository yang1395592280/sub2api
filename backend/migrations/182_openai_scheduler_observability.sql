ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS e2e_first_token_ms INTEGER,
  ADD COLUMN IF NOT EXISTS routing_ms INTEGER,
  ADD COLUMN IF NOT EXISTS queue_ms INTEGER,
  ADD COLUMN IF NOT EXISTS retry_ms INTEGER;

CREATE INDEX IF NOT EXISTS idx_usage_logs_e2e_ttft_created_at
  ON usage_logs (created_at DESC)
  WHERE e2e_first_token_ms IS NOT NULL;
