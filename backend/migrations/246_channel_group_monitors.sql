-- V1 分组账号抽测：与旧外部 API 监控独立，升级不启用任何新计划。
CREATE TABLE IF NOT EXISTS channel_group_monitors (
 id BIGSERIAL PRIMARY KEY,
 group_id BIGINT NOT NULL UNIQUE REFERENCES groups(id) ON DELETE CASCADE,
 config JSONB NOT NULL,
 next_probe_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 next_pelican_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 lease_until TIMESTAMPTZ,
 lease_token VARCHAR(64) NOT NULL DEFAULT '',
 budget_day DATE NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')::date,
 requests_used INTEGER NOT NULL DEFAULT 0,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS channel_group_monitor_runs (
 id BIGSERIAL PRIMARY KEY,
 monitor_id BIGINT NOT NULL REFERENCES channel_group_monitors(id) ON DELETE CASCADE,
 lease_token VARCHAR(64) NOT NULL,
 kind VARCHAR(16) NOT NULL CHECK (kind IN ('probe', 'pelican')),
 model VARCHAR(255) NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'running',
 detail JSONB NOT NULL DEFAULT '{}',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_channel_group_monitor_runs_history ON channel_group_monitor_runs (monitor_id, id DESC);
