CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_e2e_ttft_created_at
  ON usage_logs (created_at DESC)
  WHERE e2e_first_token_ms IS NOT NULL;
