-- Run only with every application instance stopped, after the active multi-question
-- queue has drained. New tables, question snapshots and historical records remain.
BEGIN;
LOCK TABLE account_pelican_tests IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM account_pelican_tests WHERE status IN ('queued','running')
    GROUP BY account_id,test_kind HAVING COUNT(*)>1
  ) THEN
    RAISE EXCEPTION 'Active multi-question jobs remain; drain them before rollback';
  END IF;
END $$;
UPDATE group_pelican_test_schedules
SET enabled=FALSE, interval_minutes=30, next_run_at=NULL, updated_at=NOW();
ALTER TABLE group_pelican_test_schedules
  DROP CONSTRAINT group_pelican_test_schedules_interval_minutes_check,
  ADD CONSTRAINT group_pelican_test_schedules_interval_minutes_check CHECK(interval_minutes=30);
DROP INDEX idx_account_pelican_tests_active;
CREATE UNIQUE INDEX idx_account_pelican_tests_active
  ON account_pelican_tests(account_id,test_kind) WHERE status IN ('queued','running');
COMMIT;
