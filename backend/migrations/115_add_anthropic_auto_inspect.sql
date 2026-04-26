CREATE TABLE IF NOT EXISTS anthropic_auto_inspect_batches (
    id BIGSERIAL PRIMARY KEY,
    trigger_source TEXT NOT NULL,
    status TEXT NOT NULL,
    total_accounts INTEGER NOT NULL DEFAULT 0,
    processed_accounts INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    rate_limited_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS anthropic_auto_inspect_logs (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES anthropic_auto_inspect_batches(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL,
    account_name_snapshot TEXT NOT NULL,
    platform TEXT NOT NULL,
    account_type TEXT NOT NULL,
    result TEXT NOT NULL,
    skip_reason TEXT NOT NULL DEFAULT '',
    response_text TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    rate_limit_reset_at TIMESTAMPTZ,
    temp_unschedulable_until TIMESTAMPTZ,
    schedulable_changed BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_anthropic_auto_inspect_logs_created_at
    ON anthropic_auto_inspect_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_anthropic_auto_inspect_logs_batch_id
    ON anthropic_auto_inspect_logs (batch_id);
