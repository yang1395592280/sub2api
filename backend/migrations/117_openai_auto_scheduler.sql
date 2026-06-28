ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS openai_auto_scheduler_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS openai_auto_scheduler_score_states (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  model VARCHAR(200) NOT NULL DEFAULT '',
  final_score INTEGER NOT NULL DEFAULT 6000,
  base_score INTEGER NOT NULL DEFAULT 6000,
  latency_score INTEGER NOT NULL DEFAULT 0,
  error_score INTEGER NOT NULL DEFAULT 0,
  recovery_score INTEGER NOT NULL DEFAULT 0,
  cost_score INTEGER NOT NULL DEFAULT 0,
  state VARCHAR(20) NOT NULL DEFAULT 'running',
  consecutive_slow_count INTEGER NOT NULL DEFAULT 0,
  consecutive_error_count INTEGER NOT NULL DEFAULT 0,
  consecutive_success_count INTEGER NOT NULL DEFAULT 0,
  request_count BIGINT NOT NULL DEFAULT 0,
  ttfb_sample_count BIGINT NOT NULL DEFAULT 0,
  slow_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  stuck_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  cooldown_until TIMESTAMPTZ NULL,
  last_latency_ms INTEGER NULL,
  last_ttfb_ms INTEGER NULL,
  last_status_code INTEGER NULL,
  last_error TEXT NULL,
  reason TEXT NOT NULL DEFAULT '',
  last_checked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT openai_auto_scheduler_score_states_score_check
    CHECK (final_score >= 0 AND final_score <= 10000)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_key
  ON openai_auto_scheduler_score_states (account_id, group_id, model);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_group_score
  ON openai_auto_scheduler_score_states (group_id, final_score);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_group_state
  ON openai_auto_scheduler_score_states (group_id, state);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_cooldown
  ON openai_auto_scheduler_score_states (cooldown_until);

CREATE TABLE IF NOT EXISTS openai_auto_scheduler_score_events (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  model VARCHAR(200) NOT NULL DEFAULT '',
  event_type VARCHAR(40) NOT NULL,
  score_before INTEGER NOT NULL,
  score_after INTEGER NOT NULL,
  latency_ms INTEGER NULL,
  ttfb_ms INTEGER NULL,
  status_code INTEGER NULL,
  message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_account_group_model_created
  ON openai_auto_scheduler_score_events (account_id, group_id, model, created_at);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_group_created
  ON openai_auto_scheduler_score_events (group_id, created_at);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_type_created
  ON openai_auto_scheduler_score_events (event_type, created_at);
