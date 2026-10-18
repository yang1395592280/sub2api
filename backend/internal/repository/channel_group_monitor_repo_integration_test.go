//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMonitorLeaseBudgetAndStaleWrites(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group, err := client.Group.Create().SetName(fmt.Sprintf("group-monitor-%d", time.Now().UnixNano())).SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id=$1", group.ID)
	})
	repo := NewGroupMonitorRepository(integrationDB)
	c := service.GroupMonitorConfig{Models: []string{"a", "b"}, Enabled: true, ProbeInterval: 300, PelicanInterval: 3600, PelicanPrompt: "draw", DailyLimit: 4, KeepHistory: 10}
	require.NoError(t, repo.SaveBatch(ctx, []int64{group.ID}, c))
	plans, err := repo.List(ctx)
	require.NoError(t, err)
	var id int64
	for _, p := range plans {
		if p.GroupID == group.ID {
			id = p.ID
		}
	}
	require.NotZero(t, id)
	var wg sync.WaitGroup
	claims := make(chan *service.GroupMonitorClaim, 2)
	for _, token := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			claim, e := repo.Claim(ctx, id, "probe", token, true)
			if e == nil {
				claims <- claim
			}
		}(token)
	}
	wg.Wait()
	close(claims)
	require.Len(t, claims, 1)
	claim := <-claims
	_, err = repo.Claim(ctx, id, "pelican", "manual", true)
	require.ErrorIs(t, err, service.ErrGroupMonitorBusy)
	run := claim.Runs[0]
	run.Status = "success"
	run.Detail.UpstreamMessageID = "original"
	require.NoError(t, repo.FinishRun(ctx, id, claim.Token, run))
	require.NoError(t, repo.Release(ctx, id, claim.Token, 10))
	second, err := repo.Claim(ctx, id, "pelican", "next-worker", true)
	require.NoError(t, err)
	run.Status = "failed"
	require.NoError(t, repo.FinishRun(ctx, id, claim.Token, run))
	history, err := repo.History(ctx, id, "probe", "a", 10)
	require.NoError(t, err)
	require.Equal(t, "success", history[0].Status)
	require.Equal(t, "original", history[0].Detail.UpstreamMessageID)
	require.NoError(t, repo.Release(ctx, id, second.Token, 10))
	_, err = repo.Claim(ctx, id, "probe", "over-budget", true)
	require.ErrorIs(t, err, service.ErrGroupMonitorBusy)
	// Disabling a schedule leaves its prior result and due timestamps intact.
	c.Enabled = false
	require.NoError(t, repo.SaveBatch(ctx, []int64{group.ID}, c))
	_, err = repo.Claim(ctx, id, "probe", "paused", false)
	require.ErrorIs(t, err, service.ErrGroupMonitorBusy)
}

func TestGroupMonitorHistoryReviewRetentionAndRecovery(t *testing.T) {
	ctx := context.Background()
	group, err := testEntClient(t).Group.Create().SetName(fmt.Sprintf("group-monitor-history-%d", time.Now().UnixNano())).SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id=$1", group.ID)
	})
	repo := NewGroupMonitorRepository(integrationDB)
	c := service.GroupMonitorConfig{Models: []string{"a"}, Enabled: true, ProbeInterval: 300, PelicanInterval: 3600, PelicanPrompt: "draw", DailyLimit: 100, KeepHistory: 10}
	require.NoError(t, repo.SaveBatch(ctx, []int64{group.ID}, c))
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM channel_group_monitors WHERE group_id=$1", group.ID).Scan(&id))
	claim, err := repo.Claim(ctx, id, "pelican", "pelican", true)
	require.NoError(t, err)
	run := claim.Runs[0]
	run.Status = "success"
	run.Detail.HTML = "<html><body>pelican</body></html>"
	run.Detail.Review = "unreviewed"
	require.NoError(t, repo.FinishRun(ctx, id, claim.Token, run))
	require.NoError(t, repo.Release(ctx, id, claim.Token, 10))
	require.NoError(t, repo.Review(ctx, id, run.ID, "pass"))
	history, err := repo.History(ctx, id, "pelican", "a", 500)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Empty(t, history[0].Detail.HTML, "history must remain a summary")
	require.Equal(t, "pass", history[0].Detail.Review)
	full, err := repo.GetRun(ctx, id, run.ID)
	require.NoError(t, err)
	require.Equal(t, run.Detail.HTML, full.Detail.HTML)
	plans, err := repo.List(ctx)
	require.NoError(t, err)
	for _, p := range plans {
		for _, recent := range p.LatestRuns {
			require.Empty(t, recent.Detail.HTML)
		}
	}
	// Frequent cheap probes must not evict the last pelican preview.
	for i := 0; i < 12; i++ {
		probe, e := repo.Claim(ctx, id, "probe", fmt.Sprintf("probe-%d", i), true)
		require.NoError(t, e)
		r := probe.Runs[0]
		r.Status = "success"
		require.NoError(t, repo.FinishRun(ctx, id, probe.Token, r))
		require.NoError(t, repo.Release(ctx, id, probe.Token, 10))
	}
	history, err = repo.History(ctx, id, "probe", "a", 500)
	require.NoError(t, err)
	require.Len(t, history, 10)
	_, err = repo.GetRun(ctx, id, run.ID)
	require.NoError(t, err)
	interrupted, err := repo.Claim(ctx, id, "probe", "interrupted", true)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE channel_group_monitor_runs SET created_at=NOW()-INTERVAL '21 minutes' WHERE id=$1", interrupted.Runs[0].ID)
	require.NoError(t, err)
	require.NoError(t, repo.Recover(ctx))
	full, err = repo.GetRun(ctx, id, interrupted.Runs[0].ID)
	require.NoError(t, err)
	require.Equal(t, "failed", full.Status)
	require.Contains(t, full.Detail.Error, "超时")
	// A previous UTC day's reserved budget does not block a new day's work.
	_, err = integrationDB.ExecContext(ctx, "UPDATE channel_group_monitors SET budget_day=(NOW() AT TIME ZONE 'UTC')::date-1,requests_used=100,lease_until=NOW()-INTERVAL '1 second' WHERE id=$1", id)
	require.NoError(t, err)
	next, err := repo.Claim(ctx, id, "probe", "new-day", true)
	require.NoError(t, err)
	stale := next.Runs[0]
	stale.Status = "success"
	require.NoError(t, repo.FinishRun(ctx, id, interrupted.Token, stale))
	full, err = repo.GetRun(ctx, id, stale.ID)
	require.NoError(t, err)
	require.Equal(t, "running", full.Status, "stale workers cannot finish a current worker's run")
}
