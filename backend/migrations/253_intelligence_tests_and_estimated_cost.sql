-- Existing tasks and schedules remain pelican tests. New schedules are opt-in.
ALTER TABLE account_pelican_tests
    ADD COLUMN test_kind VARCHAR(24) NOT NULL DEFAULT 'pelican' CHECK (test_kind IN ('pelican', 'intelligence')),
    ADD COLUMN answer TEXT NOT NULL DEFAULT '',
    ADD COLUMN estimated_cost_usd NUMERIC(24,12),
    ADD COLUMN upstream_rate_multiplier NUMERIC(20,10),
    ADD COLUMN cost_model_id VARCHAR(255) NOT NULL DEFAULT '';

DROP INDEX idx_account_pelican_tests_active;
CREATE UNIQUE INDEX idx_account_pelican_tests_active
    ON account_pelican_tests (account_id, test_kind)
    WHERE status IN ('queued', 'running');
CREATE INDEX idx_account_pelican_tests_kind_latest
    ON account_pelican_tests (account_id, test_kind, id DESC);

ALTER TABLE group_pelican_test_schedules
    ADD COLUMN test_kind VARCHAR(24) NOT NULL DEFAULT 'pelican' CHECK (test_kind IN ('pelican', 'intelligence'));
ALTER TABLE group_pelican_test_schedules DROP CONSTRAINT group_pelican_test_schedules_pkey;
ALTER TABLE group_pelican_test_schedules ADD PRIMARY KEY (group_id, test_kind);
