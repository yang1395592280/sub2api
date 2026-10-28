-- Independent evidence for automatic no-degradation routing. Forward only.
CREATE TABLE account_quality_policies (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    source_group_id BIGINT NOT NULL,
    control_scope_hash VARCHAR(64) NOT NULL,
    scope_hash VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    policy_control_epoch BIGINT NOT NULL DEFAULT 0,
    eligibility_status VARCHAR(32) NOT NULL DEFAULT 'eligible',
    document JSONB NOT NULL,
    next_run_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX account_quality_active_scope ON account_quality_policies(account_id, control_scope_hash) WHERE archived_at IS NULL;
CREATE INDEX account_quality_due ON account_quality_policies(next_run_at) WHERE archived_at IS NULL;
CREATE INDEX account_quality_account ON account_quality_policies(account_id, id DESC);

CREATE TABLE account_quality_test_runs (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    policy_id BIGINT NOT NULL REFERENCES account_quality_policies(id),
    account_id BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    trigger_source VARCHAR(24) NOT NULL,
    actor_user_id BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(24) NOT NULL DEFAULT 'queued',
    verdict VARCHAR(24),
    reason_code VARCHAR(64) NOT NULL DEFAULT '',
    scheduled_slot TIMESTAMPTZ,
    lease_token VARCHAR(36),
    lease_expires_at TIMESTAMPTZ,
    cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
    upstream_dispatched_at TIMESTAMPTZ,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    queue_expires_at TIMESTAMPTZ NOT NULL,
    result JSONB NOT NULL DEFAULT '{}',
    response_text TEXT,
    response_expires_at TIMESTAMPTZ,
    state_applied BOOLEAN NOT NULL DEFAULT FALSE,
    state_skip_reason VARCHAR(64) NOT NULL DEFAULT '',
    budget_date DATE,
    budget_reserved BOOLEAN NOT NULL DEFAULT FALSE,
    budget_consumed BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (status IN ('queued','running','completed','cancelled','interrupted','skipped')),
    CHECK (verdict IS NULL OR verdict IN ('passed','incorrect','unknown','error'))
);
CREATE UNIQUE INDEX account_quality_active_run ON account_quality_test_runs(policy_id) WHERE status IN ('queued','running');
CREATE UNIQUE INDEX account_quality_running_account ON account_quality_test_runs(account_id) WHERE status='running';
CREATE UNIQUE INDEX account_quality_scheduled_slot ON account_quality_test_runs(policy_id,scheduled_slot) WHERE scheduled_slot IS NOT NULL;
CREATE INDEX account_quality_run_policy ON account_quality_test_runs(policy_id,id DESC);
CREATE INDEX account_quality_run_account ON account_quality_test_runs(account_id,id DESC);
CREATE INDEX account_quality_run_batch ON account_quality_test_runs(batch_id);
CREATE INDEX account_quality_run_monitor ON account_quality_test_runs(policy_id, COALESCE(started_at,queued_at));
CREATE INDEX account_quality_run_queue ON account_quality_test_runs(status,queued_at);

CREATE TABLE account_quality_events (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT REFERENCES account_quality_policies(id),
    account_id BIGINT,
    run_id BIGINT REFERENCES account_quality_test_runs(id) ON DELETE SET NULL,
    event_type VARCHAR(64) NOT NULL,
    document JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX account_quality_event_policy ON account_quality_events(policy_id,id DESC);
CREATE INDEX account_quality_event_account ON account_quality_events(account_id,id DESC);

CREATE TABLE account_quality_daily_budgets (
    budget_date DATE NOT NULL,
    scope_type VARCHAR(16) NOT NULL CHECK(scope_type IN ('global','policy')),
    scope_id BIGINT NOT NULL,
    reserved_calls INTEGER NOT NULL DEFAULT 0 CHECK(reserved_calls>=0),
    consumed_calls INTEGER NOT NULL DEFAULT 0 CHECK(consumed_calls>=0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(budget_date,scope_type,scope_id)
);

INSERT INTO settings(key,value,updated_at) VALUES ('openai_account_quality',
 '{"revision":1,"control_epoch":0,"automation_enabled":false,"control_enabled":false,"global_concurrency":3,"queue_limit":300,"queue_timeout_seconds":300,"global_daily_call_limit":1000,"default_policy_daily_call_limit":100,"raw_retention_days":7,"metadata_retention_days":30,"store_failure_policy":"fail_closed"}', NOW())
ON CONFLICT(key) DO NOTHING;
