-- Each selected account gets one independently tracked generation result.
CREATE TABLE IF NOT EXISTS account_pelican_tests (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    -- Keep the selected ID when an account is deleted during an active batch so
    -- that its task can finish as failed without blocking the other accounts.
    account_id BIGINT NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    prompt TEXT NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'queued',
    response_text TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_account_pelican_tests_latest
    ON account_pelican_tests (account_id, id DESC);

CREATE INDEX IF NOT EXISTS idx_account_pelican_tests_created
    ON account_pelican_tests (created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_pelican_tests_active
    ON account_pelican_tests (account_id)
    WHERE status IN ('queued', 'running');
