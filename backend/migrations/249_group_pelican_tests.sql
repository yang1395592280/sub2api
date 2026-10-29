-- Separate from legacy quality monitoring: these jobs only generate previews.
CREATE TABLE group_pelican_test_schedules (
    group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    model_id VARCHAR(255) NOT NULL DEFAULT '',
    prompt TEXT NOT NULL,
    interval_minutes INTEGER NOT NULL DEFAULT 30 CHECK (interval_minutes = 30),
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_group_pelican_test_schedules_due
    ON group_pelican_test_schedules (next_run_at) WHERE enabled;

-- No FK: historical attribution survives group deletion and renaming.
ALTER TABLE account_pelican_tests
    ADD COLUMN source_group_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN source_group_name TEXT NOT NULL DEFAULT '';
