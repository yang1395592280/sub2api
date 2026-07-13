CREATE TABLE IF NOT EXISTS openai_scheduler_health_states (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  model_family VARCHAR(100) NOT NULL DEFAULT '',
  endpoint VARCHAR(100) NOT NULL DEFAULT '',
  transport VARCHAR(32) NOT NULL DEFAULT '',
  state VARCHAR(20) NOT NULL DEFAULT 'running',
  predicted_ttft_ms DECIMAL(12,3) NOT NULL DEFAULT 0,
  error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  rate_limited_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  server_error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  consecutive_slow INTEGER NOT NULL DEFAULT 0,
  consecutive_error INTEGER NOT NULL DEFAULT 0,
  consecutive_success INTEGER NOT NULL DEFAULT 0,
  real_sample_count BIGINT NOT NULL DEFAULT 0,
  probe_sample_count BIGINT NOT NULL DEFAULT 0,
  last_real_at TIMESTAMPTZ,
  last_probe_at TIMESTAMPTZ,
  cooldown_until TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_openai_scheduler_health_key
  ON openai_scheduler_health_states(account_id, model_family, endpoint, transport);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_health_expiry
  ON openai_scheduler_health_states(expires_at);
