CREATE TABLE IF NOT EXISTS openai_scheduler_decision_audits (
  id BIGSERIAL PRIMARY KEY,
  event_type VARCHAR(40) NOT NULL,
  group_id BIGINT NOT NULL DEFAULT 0,
  account_id BIGINT NOT NULL DEFAULT 0,
  legacy_account_id BIGINT NOT NULL DEFAULT 0,
  model_family VARCHAR(100) NOT NULL DEFAULT '',
  endpoint VARCHAR(100) NOT NULL DEFAULT '',
  transport VARCHAR(32) NOT NULL DEFAULT '',
  reason VARCHAR(100) NOT NULL DEFAULT '',
  confidence VARCHAR(20) NOT NULL DEFAULT '',
  eligibility VARCHAR(32) NOT NULL DEFAULT '',
  traffic_class VARCHAR(20) NOT NULL DEFAULT '',
  predicted_ttft_difference_ms DECIMAL(12,3) NOT NULL DEFAULT 0,
  target_share DECIMAL(8,6) NOT NULL DEFAULT 0,
  candidate_count INTEGER NOT NULL DEFAULT 0,
  top_k INTEGER NOT NULL DEFAULT 0,
  scheduler_mode VARCHAR(32) NOT NULL DEFAULT '',
  shadow_mode BOOLEAN NOT NULL DEFAULT FALSE,
  exploration_rate DECIMAL(8,6) NOT NULL DEFAULT 0,
  exploration_budget DECIMAL(8,6) NOT NULL DEFAULT 0,
  low_confidence_max_share DECIMAL(8,6) NOT NULL DEFAULT 0,
  latency_weight DECIMAL(8,6) NOT NULL DEFAULT 0,
  reliability_weight DECIMAL(8,6) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_decision_audits_created
  ON openai_scheduler_decision_audits(created_at);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_decision_audits_type_created
  ON openai_scheduler_decision_audits(event_type, created_at);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_decision_audits_group_created
  ON openai_scheduler_decision_audits(group_id, created_at);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_decision_audits_account_created
  ON openai_scheduler_decision_audits(account_id, created_at);
