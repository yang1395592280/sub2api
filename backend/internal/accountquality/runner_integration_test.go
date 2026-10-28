//go:build integration

package accountquality

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Uses only an explicitly supplied disposable database. Each invocation owns
// a random schema; no production tables or account credentials are needed.
func TestDualBudgetLifecyclePostgres(t *testing.T) {
	db, store, s, policy := prepareDualDatabase(t, 3)
	ctx := context.Background()
	run, err := s.claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, run)
	require.NoError(t, s.dispatch(ctx, *run, "candy"))
	require.Error(t, s.dispatch(ctx, *run, "candy"), "a committed stage cannot be replayed")
	require.NoError(t, s.dispatch(ctx, *run, "pelican"))
	// Cancellation after a paid request must stop the next paid stage.
	_, err = db.Exec(`UPDATE account_quality_test_runs SET cancel_requested=TRUE WHERE id=$1`, run.ID)
	require.NoError(t, err)
	require.Error(t, s.dispatch(ctx, *run, "review"))
	_, err = db.Exec(`UPDATE account_quality_test_runs SET cancel_requested=FALSE WHERE id=$1`, run.ID)
	require.NoError(t, err)
	require.NoError(t, s.dispatch(ctx, *run, "review"))
	require.Error(t, s.dispatch(ctx, *run, "review"))
	verifyDualBudgetLifecycle(t, db, store, s, policy, *run)
}

func prepareDualDatabase(t *testing.T, limit int) (*sql.DB, *Store, *Service, Policy) {
	t.Helper()
	dsn := os.Getenv("QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("QUALITY_TEST_DSN required")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	schema := "quality_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = db.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	_, err = db.Exec(`SET search_path TO ` + schema)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT,updated_at TIMESTAMPTZ)`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/248_account_quality_control.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	settings := DefaultSettings()
	settings.GlobalDailyCallLimit = limit
	store := NewStore(db, "Asia/Shanghai")
	s := NewService(store, nil)
	ctx := context.Background()
	var policy Policy
	_, err = store.Transaction(ctx, false, func(tx *Tx) (any, error) {
		if err := tx.SaveSettings(settings); err != nil {
			return nil, err
		}
		policy = Policy{AccountID: 42, SourceGroupID: 7, Condition: Condition{BenchmarkID: "dual", BenchmarkVersion: "1", ScorerVersion: "dual-visual-v1"}, Rules: DefaultRules(limit), Eligibility: "eligible", Revision: 1, ControlScopeHash: "control", ScopeHash: "scope", State: NewState(0, 0)}
		if err := tx.SavePolicy(&policy); err != nil {
			return nil, err
		}
		return tx.Enqueue(policy, newBatchID(), "manual", 1, nil)
	})
	require.NoError(t, err)
	return db, store, s, policy
}

func verifyDualBudgetLifecycle(t *testing.T, db *sql.DB, store *Store, s *Service, policy Policy, run Run) {
	t.Helper()
	ctx := context.Background()
	_, err := store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		g, e := tx.Budget("global", 0)
		require.NoError(t, e)
		require.Equal(t, 3, g.Consumed)
		require.Zero(t, g.Reserved)
		local, e := tx.Budget("policy", policy.ID)
		require.NoError(t, e)
		require.Equal(t, 3, local.Consumed)
		return nil, nil
	})
	require.NoError(t, err)
	require.NoError(t, s.finish(ctx, run, Combine(&CheckResult{Verdict: "passed"}, &CheckResult{Verdict: "passed", Previewable: true}, &CheckResult{Verdict: "passed"}, &Evidence{Candy: `{"answer":21}`, SVG: `<svg/>`})))
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		r, e := tx.Run(run.ID, false)
		require.NoError(t, e)
		require.Equal(t, "completed", r.Status)
		require.Equal(t, "passed", *r.Verdict)
		require.Equal(t, []string{"candy", "pelican", "review"}, r.Result.DispatchedStages)
		list, _, e := tx.Runs(RunFilter{Limit: 10})
		require.NoError(t, e)
		require.Nil(t, list[0].ResponseText)
		require.Nil(t, list[0].Evidence)
		gallery, _, e := tx.Runs(RunFilter{Limit: 10, IncludeEvidence: true, BenchmarkID: "dual"})
		require.NoError(t, e)
		require.Equal(t, "<svg/>", gallery[0].Evidence.SVG)
		require.Empty(t, gallery[0].Evidence.Candy)
		return nil, nil
	})
	require.NoError(t, err)
	// Expiry applies immediately to reads, even before the periodic cleanup.
	_, err = db.Exec(`UPDATE account_quality_test_runs SET response_expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, run.ID)
	require.NoError(t, err)
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		items, _, e := tx.Runs(RunFilter{Limit: 10, IncludeEvidence: true})
		require.NoError(t, e)
		require.Nil(t, items[0].Evidence)
		return nil, nil
	})
	require.NoError(t, err)
	// Budget exhaustion creates no additional sent calls.
	_, err = store.Transaction(ctx, false, func(tx *Tx) (any, error) {
		sub, e := tx.Enqueue(policy, newBatchID(), "manual", 1, nil)
		require.NoError(t, e)
		require.Equal(t, "QUALITY_DAILY_BUDGET_EXCEEDED", sub.Reason)
		return nil, nil
	})
	require.NoError(t, err)
	// Reviewer changes invalidate previously accepted evidence.
	s.provider = reviewerIntegrationProvider{}
	_, err = store.Transaction(ctx, false, func(tx *Tx) (any, error) {
		next := tx.Settings
		next.Reviewer = &ReviewerConfig{AccountID: 99, SourceGroupID: 7, Condition: Condition{ModelID: "review", Protocol: "responses", Transport: "http", ReasoningEffort: "upstream_default", ServiceTier: "upstream_default", BenchmarkID: "candy", BenchmarkVersion: "1", ScorerVersion: "strict-json-v1"}}
		_, e := s.ChangeSettings(tx, tx.Settings.Revision, next, 1, false, "reviewer_changed")
		return nil, e
	})
	require.NoError(t, err)
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		require.EqualValues(t, 1, tx.Settings.ControlEpoch)
		p, e := tx.Policy(policy.ID, false)
		require.NoError(t, e)
		require.Equal(t, "untested", Present(p, tx.Settings, tx.Now).State.Name)
		return nil, nil
	})
	require.NoError(t, err)
}

func TestDualStopsAtMidSuiteBudgetLimit(t *testing.T) {
	_, store, s, _ := prepareDualDatabase(t, 1)
	ctx := context.Background()
	run, err := s.claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, run)
	require.NoError(t, s.dispatch(ctx, *run, "candy"))
	require.Error(t, s.dispatch(ctx, *run, "pelican"))
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		budget, e := tx.Budget("global", 0)
		require.NoError(t, e)
		require.Equal(t, 1, budget.Consumed)
		require.Zero(t, budget.Reserved)
		r, e := tx.Run(run.ID, false)
		require.NoError(t, e)
		require.Equal(t, []string{"candy"}, r.Result.DispatchedStages)
		return nil, nil
	})
	require.NoError(t, err)
}

func TestDualRejectsExpiredLeaseAndReleasesReservation(t *testing.T) {
	db, store, s, _ := prepareDualDatabase(t, 3)
	ctx := context.Background()
	run, err := s.claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, run)
	_, err = db.Exec(`UPDATE account_quality_test_runs SET lease_expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, run.ID)
	require.NoError(t, err)
	require.Error(t, s.dispatch(ctx, *run, "candy"))
	require.NoError(t, s.tick(ctx, false))
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		budget, e := tx.Budget("global", 0)
		require.NoError(t, e)
		require.Zero(t, budget.Consumed)
		require.Zero(t, budget.Reserved)
		r, e := tx.Run(run.ID, false)
		require.NoError(t, e)
		require.Equal(t, "interrupted", r.Status)
		return nil, nil
	})
	require.NoError(t, err)
}

func TestDualMovesPreviousDayReservationToSendDay(t *testing.T) {
	_, store, s, _ := prepareDualDatabase(t, 3)
	ctx := context.Background()
	run, err := s.claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, run)
	// Simulate a claim just before midnight, without changing the machine clock.
	_, err = store.Transaction(ctx, false, func(tx *Tx) (any, error) {
		_, e := tx.sql.ExecContext(ctx, `UPDATE account_quality_daily_budgets SET budget_date=budget_date-1`)
		if e != nil {
			return nil, e
		}
		_, e = tx.sql.ExecContext(ctx, `UPDATE account_quality_test_runs SET budget_date=budget_date-1 WHERE id=$1`, run.ID)
		return nil, e
	})
	require.NoError(t, err)
	require.NoError(t, s.dispatch(ctx, *run, "candy"))
	_, err = store.Transaction(ctx, true, func(tx *Tx) (any, error) {
		g, e := tx.Budget("global", 0)
		require.NoError(t, e)
		require.Equal(t, 1, g.Consumed)
		require.Zero(t, g.Reserved)
		var oldReserved, oldConsumed int
		e = tx.sql.QueryRowContext(ctx, `SELECT reserved_calls,consumed_calls FROM account_quality_daily_budgets WHERE budget_date=$1::date-1 AND scope_type='global' AND scope_id=0`, g.Date).Scan(&oldReserved, &oldConsumed)
		require.NoError(t, e)
		require.Zero(t, oldReserved)
		require.Zero(t, oldConsumed)
		return nil, nil
	})
	require.NoError(t, err)
}

type reviewerIntegrationProvider struct{}

func (reviewerIntegrationProvider) Targets(context.Context) ([]Target, error) { return nil, nil }
func (reviewerIntegrationProvider) Resolve(context.Context, int64, int64, Condition) (Target, error) {
	return Target{CanProbe: true}, nil
}
func (reviewerIntegrationProvider) Probe(context.Context, Run, func(string) error) Result {
	panic("not used")
}
