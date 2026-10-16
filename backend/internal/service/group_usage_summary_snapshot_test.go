package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestGroupUsageSummarySnapshotPreservesCacheTimestampAndCopiesItems(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	repo := &usageRepoStub{groupUsageSummaries: [][]usagestats.GroupUsageSummary{{{GroupID: 1, TodayCost: 2}}}}
	svc := NewDashboardService(repo, nil, nil, nil)
	first, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
	require.NoError(t, err)
	require.False(t, first.UpdatedAt.IsZero())
	first.Items[0].TodayCost = 99
	second, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
	require.NoError(t, err)
	require.Equal(t, first.UpdatedAt, second.UpdatedAt)
	require.Equal(t, float64(2), second.Items[0].TodayCost)
	require.False(t, second.Stale)
	require.False(t, second.RefreshFailed)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
}

func TestGroupUsageSummarySnapshotManualRefreshBypassesFreshCache(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	repo := &usageRepoStub{groupUsageSummaries: [][]usagestats.GroupUsageSummary{
		{{GroupID: 1, TodayCost: 2}}, {{GroupID: 1, TodayCost: 3}},
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	first, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
	require.NoError(t, err)
	second, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, true)
	require.NoError(t, err)
	require.Equal(t, float64(3), second.Items[0].TodayCost)
	require.False(t, second.UpdatedAt.Before(first.UpdatedAt))
	require.Equal(t, int32(2), atomic.LoadInt32(&repo.groupUsageSummaryCall))
	cached, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
	require.NoError(t, err)
	require.Equal(t, second, cached)
}

func TestGroupUsageSummarySnapshotManualRefreshSharesBackgroundQuery(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	started := make(chan struct{})
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(ctx context.Context, _ time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		close(started)
		select {
		case <-release:
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	updatedAt := time.Now().Add(-time.Minute)
	svc.storeGroupUsageSummary(groupUsageSummaryCacheKey(today), []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2}}, updatedAt)
	stale, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
	require.NoError(t, err)
	require.True(t, stale.Stale)
	require.Equal(t, updatedAt, stale.UpdatedAt)
	<-started
	// 手动读取应等待正在进行的官方查询，而不是拿旧值冒充刷新成功。
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = svc.GetGroupUsageSummarySnapshot(ctx, today, true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	close(release)
	require.Eventually(t, func() bool {
		current, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, false)
		return err == nil && !current.Stale && current.Items[0].TodayCost == 3
	}, time.Second, time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
}

func TestGroupUsageSummarySnapshotRefreshFailureKeepsTimestampAndRecovers(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	repo := &usageRepoStub{groupUsageSummaryFn: func(_ context.Context, _ time.Time, call int32) ([]usagestats.GroupUsageSummary, error) {
		if call == 1 {
			return nil, errors.New("database unavailable")
		}
		return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3}}, nil
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	updatedAt := time.Now().Add(-time.Second)
	key := groupUsageSummaryCacheKey(today)
	svc.storeGroupUsageSummary(key, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2}}, updatedAt)
	failed, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, true)
	require.NoError(t, err)
	require.Equal(t, updatedAt, failed.UpdatedAt)
	require.Equal(t, float64(2), failed.Items[0].TodayCost)
	require.True(t, failed.Stale)
	require.True(t, failed.RefreshFailed)
	cooledDown, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, true)
	require.NoError(t, err)
	require.True(t, cooledDown.RefreshFailed)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
	svc.groupUsageSummaryMu.Lock()
	entry := svc.groupUsageSummaryCache[key]
	entry.lastRefreshAttempt = time.Now().Add(-time.Minute)
	svc.groupUsageSummaryCache[key] = entry
	svc.groupUsageSummaryMu.Unlock()
	recovered, err := svc.GetGroupUsageSummarySnapshot(context.Background(), today, true)
	require.NoError(t, err)
	require.False(t, recovered.RefreshFailed)
	require.False(t, recovered.Stale)
	require.Equal(t, float64(3), recovered.Items[0].TodayCost)
	require.True(t, recovered.UpdatedAt.After(updatedAt))
}

func TestGroupUsageSummarySnapshotColdFailureReturnsError(t *testing.T) {
	svc := NewDashboardService(&usageRepoStub{groupUsageSummaryErr: errors.New("database unavailable")}, nil, nil, nil)
	_, err := svc.GetGroupUsageSummarySnapshot(context.Background(), time.Now(), true)
	require.ErrorContains(t, err, "database unavailable")
}
