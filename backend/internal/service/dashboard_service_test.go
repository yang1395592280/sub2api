package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type groupUserUsageDashboardRepoStub struct {
	UsageLogRepository

	stats map[string]map[int64]*usagestats.AccountStats
}

func (s *groupUserUsageDashboardRepoStub) GetGroupUserDailyStatsBatch(_ context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	key := startTime.Format(time.RFC3339) + "|" + endTime.Format(time.RFC3339)
	out := make(map[int64]*usagestats.AccountStats, len(userIDs))
	for _, userID := range userIDs {
		if s.stats != nil && s.stats[key] != nil && s.stats[key][userID] != nil {
			out[userID] = s.stats[key][userID]
		} else {
			out[userID] = &usagestats.AccountStats{}
		}
	}
	_ = groupID
	return out, nil
}

type usageRepoStub struct {
	UsageLogRepository
	stats                 *usagestats.DashboardStats
	rangeStats            *usagestats.DashboardStats
	groupUsageSummaries   [][]usagestats.GroupUsageSummary
	groupUsageSummaryFn   func(ctx context.Context, todayStart time.Time, call int32) ([]usagestats.GroupUsageSummary, error)
	err                   error
	rangeErr              error
	groupUsageSummaryErr  error
	calls                 int32
	rangeCalls            int32
	groupUsageSummaryCall int32
	rangeStart            time.Time
	rangeEnd              time.Time
	onCall                chan struct{}
}

func (s *usageRepoStub) GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.onCall != nil {
		select {
		case s.onCall <- struct{}{}:
		default:
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.stats, nil
}

func (s *usageRepoStub) GetDashboardStatsWithRange(ctx context.Context, start, end time.Time) (*usagestats.DashboardStats, error) {
	atomic.AddInt32(&s.rangeCalls, 1)
	s.rangeStart = start
	s.rangeEnd = end
	if s.rangeErr != nil {
		return nil, s.rangeErr
	}
	if s.rangeStats != nil {
		return s.rangeStats, nil
	}
	return s.stats, nil
}

func (s *usageRepoStub) GetAllGroupUsageSummary(ctx context.Context, todayStart time.Time) ([]usagestats.GroupUsageSummary, error) {
	call := atomic.AddInt32(&s.groupUsageSummaryCall, 1)
	if s.groupUsageSummaryFn != nil {
		return s.groupUsageSummaryFn(ctx, todayStart, call)
	}
	if s.groupUsageSummaryErr != nil {
		return nil, s.groupUsageSummaryErr
	}
	if len(s.groupUsageSummaries) == 0 {
		return nil, nil
	}
	idx := int(call - 1)
	if idx >= len(s.groupUsageSummaries) {
		idx = len(s.groupUsageSummaries) - 1
	}
	return s.groupUsageSummaries[idx], nil
}

type dashboardCacheStub struct {
	get       func(ctx context.Context) (string, error)
	set       func(ctx context.Context, data string, ttl time.Duration) error
	del       func(ctx context.Context) error
	getCalls  int32
	setCalls  int32
	delCalls  int32
	lastSetMu sync.Mutex
	lastSet   string
}

func (c *dashboardCacheStub) GetDashboardStats(ctx context.Context) (string, error) {
	atomic.AddInt32(&c.getCalls, 1)
	if c.get != nil {
		return c.get(ctx)
	}
	return "", ErrDashboardStatsCacheMiss
}

func (c *dashboardCacheStub) SetDashboardStats(ctx context.Context, data string, ttl time.Duration) error {
	atomic.AddInt32(&c.setCalls, 1)
	c.lastSetMu.Lock()
	c.lastSet = data
	c.lastSetMu.Unlock()
	if c.set != nil {
		return c.set(ctx, data, ttl)
	}
	return nil
}

func (c *dashboardCacheStub) DeleteDashboardStats(ctx context.Context) error {
	atomic.AddInt32(&c.delCalls, 1)
	if c.del != nil {
		return c.del(ctx)
	}
	return nil
}

type dashboardAggregationRepoStub struct {
	watermark time.Time
	err       error
}

func (s *dashboardAggregationRepoStub) AggregateRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) RecomputeRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) GetAggregationWatermark(ctx context.Context) (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.watermark, nil
}

func (s *dashboardAggregationRepoStub) UpdateAggregationWatermark(ctx context.Context, aggregatedAt time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupAggregates(ctx context.Context, hourlyCutoff, dailyCutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupUsageLogs(ctx context.Context, cutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupUsageBillingDedup(ctx context.Context, cutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) EnsureUsageLogsPartitions(ctx context.Context, now time.Time) error {
	return nil
}

func (c *dashboardCacheStub) readLastEntry(t *testing.T) dashboardStatsCacheEntry {
	t.Helper()
	c.lastSetMu.Lock()
	data := c.lastSet
	c.lastSetMu.Unlock()

	var entry dashboardStatsCacheEntry
	err := json.Unmarshal([]byte(data), &entry)
	require.NoError(t, err)
	return entry
}

func TestDashboardService_CacheHitFresh(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     10,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	entry := dashboardStatsCacheEntry{
		Stats:     stats,
		UpdatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(entry)
	require.NoError(t, err)

	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return string(payload), nil
		},
	}
	repo := &usageRepoStub{
		stats: &usagestats.DashboardStats{TotalUsers: 99},
	}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(0), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.setCalls))
}

func TestDashboardService_CacheMiss_StoresCache(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     7,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "", ErrDashboardStatsCacheMiss
		},
	}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.setCalls))
	entry := cache.readLastEntry(t)
	require.Equal(t, stats, entry.Stats)
	require.WithinDuration(t, time.Now(), time.Unix(entry.UpdatedAt, 0), time.Second)
}

func TestDashboardService_CacheDisabled_SkipsCache(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     3,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "", nil
		},
	}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.setCalls))
}

func TestDashboardService_GetGroupUserUsageComparison(t *testing.T) {
	today := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)

	usageRepo := &groupUserUsageDashboardRepoStub{
		stats: map[string]map[int64]*usagestats.AccountStats{
			today.Format(time.RFC3339) + "|" + tomorrow.Format(time.RFC3339): {
				1: {Requests: 2, Tokens: 100, Cost: 3.75, StandardCost: 3.75, UserCost: 2.25},
			},
			yesterday.Format(time.RFC3339) + "|" + today.Format(time.RFC3339): {
				1: {Requests: 1, Tokens: 40, Cost: 1.25, StandardCost: 1.25, UserCost: 0.75},
			},
		},
	}
	svc := NewDashboardService(usageRepo, nil, nil, nil)

	got, err := svc.GetGroupUserUsageComparison(context.Background(), 10, []int64{1, 2}, today)
	require.NoError(t, err)
	require.Equal(t, int64(10), got.GroupID)
	require.Equal(t, "2026-06-26", got.Today)
	require.Equal(t, "2026-06-25", got.Yesterday)
	require.Equal(t, int64(2), got.Stats[1].Today.Requests)
	require.Equal(t, int64(1), got.Stats[1].Yesterday.Requests)
	require.Equal(t, int64(0), got.Stats[2].Today.Requests)
	require.Equal(t, int64(0), got.Stats[2].Yesterday.Requests)
}

func TestDashboardService_GetGroupUserUsageComparison_EmptyUsers(t *testing.T) {
	today := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	svc := NewDashboardService(&groupUserUsageDashboardRepoStub{}, nil, nil, nil)

	got, err := svc.GetGroupUserUsageComparison(context.Background(), 10, nil, today)
	require.NoError(t, err)
	require.Equal(t, int64(10), got.GroupID)
	require.Equal(t, "2026-06-26", got.Today)
	require.Equal(t, "2026-06-25", got.Yesterday)
	require.Empty(t, got.Stats)
}

func TestDashboardService_GroupUsageSummaryUsesFiveMinuteCache(t *testing.T) {
	todayStart := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	repo := &usageRepoStub{
		groupUsageSummaries: [][]usagestats.GroupUsageSummary{
			{{GroupID: 1, TodayCost: 2.5, TotalCost: 10}},
			{{GroupID: 1, TodayCost: 9.9, TotalCost: 99}},
		},
	}
	svc := NewDashboardService(repo, nil, nil, nil)

	first, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Len(t, first, 1)
	first[0].TodayCost = 123

	second, err := svc.GetGroupUsageSummary(context.Background(), todayStart)

	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
	require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2.5, TotalCost: 10}}, second)
}

func TestDashboardService_GroupUsageSummaryCacheSeparatesTodayStart(t *testing.T) {
	firstDay := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	repo := &usageRepoStub{
		groupUsageSummaries: [][]usagestats.GroupUsageSummary{
			{{GroupID: 1, TodayCost: 1, TotalCost: 10}},
			{{GroupID: 1, TodayCost: 2, TotalCost: 12}},
		},
	}
	svc := NewDashboardService(repo, nil, nil, nil)

	first, err := svc.GetGroupUsageSummary(context.Background(), firstDay)
	require.NoError(t, err)
	second, err := svc.GetGroupUsageSummary(context.Background(), secondDay)

	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&repo.groupUsageSummaryCall))
	require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 1, TotalCost: 10}}, first)
	require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 12}}, second)

	svc.groupUsageSummaryMu.RLock()
	defer svc.groupUsageSummaryMu.RUnlock()
	require.Len(t, svc.groupUsageSummaryCache, 1)
	_, ok := svc.groupUsageSummaryCache[groupUsageSummaryCacheKey(secondDay)]
	require.True(t, ok)
}

func TestDashboardService_GroupUsageSummaryCacheTTLDefaultsToFiveMinutes(t *testing.T) {
	svc := NewDashboardService(&usageRepoStub{}, nil, nil, nil)

	require.Equal(t, 5*time.Minute, svc.groupUsageSummaryCacheTTL)
}

func TestDashboardService_GroupUsageSummaryServesStaleWhileRefreshing(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	var startedOnce sync.Once
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(ctx context.Context, _ time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}},
		updatedAt: time.Now().Add(-6 * time.Minute),
	}

	type result struct {
		value []usagestats.GroupUsageSummary
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		value, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
		resultCh <- result{value: value, err: err}
	}()
	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}}, got.value)
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("stale cache request blocked on refresh")
	}
	<-started
	close(release)
	require.Eventually(t, func() bool {
		svc.groupUsageSummaryMu.RLock()
		defer svc.groupUsageSummaryMu.RUnlock()
		return svc.groupUsageSummaryCache[key].results[0].TotalCost == 12
	}, time.Second, 10*time.Millisecond)
}

func TestDashboardService_GroupUsageSummaryRefreshCooldown(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	repo := &usageRepoStub{groupUsageSummaryErr: errors.New("refresh failed")}
	svc := NewDashboardService(repo, nil, nil, nil)
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}},
		updatedAt: time.Now().Add(-6 * time.Minute),
	}

	got, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Equal(t, float64(10), got[0].TotalCost)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&repo.groupUsageSummaryCall) == 1
	}, time.Second, 10*time.Millisecond)
	_, err = svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Never(t, func() bool {
		return atomic.LoadInt32(&repo.groupUsageSummaryCall) > 1
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestDashboardService_GroupUsageSummaryCacheAgeForLog(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	svc := NewDashboardService(&usageRepoStub{}, nil, nil, nil)
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		updatedAt: now.Add(-6 * time.Minute),
	}

	require.Equal(t, "6m0s", svc.groupUsageSummaryCacheAgeForLog(key, now))

	delete(svc.groupUsageSummaryCache, key)
	require.Equal(t, "unknown", svc.groupUsageSummaryCacheAgeForLog(key, now))
}

func TestDashboardService_GroupUsageSummaryRefreshCooldownStartsAfterFailure(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(_ context.Context, _ time.Time, call int32) ([]usagestats.GroupUsageSummary, error) {
		if call == 1 {
			close(started)
			<-release
		}
		return nil, errors.New("refresh failed")
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	svc.groupUsageSummaryRetryCooldown = 100 * time.Millisecond
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}},
		updatedAt: time.Now().Add(-6 * time.Minute),
	}

	_, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	<-started
	time.Sleep(svc.groupUsageSummaryRetryCooldown + 20*time.Millisecond)
	failureDone := svc.groupUsageSummarySF.DoChan(fmt.Sprintf("%d", key), func() (any, error) {
		return nil, errors.New("unexpected refresh")
	})
	close(release)
	failureResult := <-failureDone
	require.ErrorContains(t, failureResult.Err, "refresh failed")

	_, err = svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Never(t, func() bool {
		return atomic.LoadInt32(&repo.groupUsageSummaryCall) > 1
	}, 50*time.Millisecond, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		_, getErr := svc.GetGroupUsageSummary(context.Background(), todayStart)
		return getErr == nil && atomic.LoadInt32(&repo.groupUsageSummaryCall) == 2
	}, time.Second, 10*time.Millisecond)
}

func TestDashboardService_GroupUsageSummaryOlderRefreshCannotReplaceNewDay(t *testing.T) {
	firstDay := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	oldStarted := make(chan struct{})
	oldRelease := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(_ context.Context, todayStart time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		if todayStart.Equal(firstDay) {
			close(oldStarted)
			<-oldRelease
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 1, TotalCost: 11}}, nil
		}
		return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 12}}, nil
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	firstKey := groupUsageSummaryCacheKey(firstDay)
	secondKey := groupUsageSummaryCacheKey(secondDay)
	svc.groupUsageSummaryCache[firstKey] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 0, TotalCost: 10}},
		updatedAt: time.Now().Add(-6 * time.Minute),
	}

	_, err := svc.GetGroupUsageSummary(context.Background(), firstDay)
	require.NoError(t, err)
	<-oldStarted
	oldDone := svc.groupUsageSummarySF.DoChan(fmt.Sprintf("%d", firstKey), func() (any, error) {
		return nil, errors.New("unexpected refresh")
	})
	second, err := svc.GetGroupUsageSummary(context.Background(), secondDay)
	require.NoError(t, err)
	require.Equal(t, float64(12), second[0].TotalCost)
	close(oldRelease)
	require.NoError(t, (<-oldDone).Err)

	svc.groupUsageSummaryMu.RLock()
	defer svc.groupUsageSummaryMu.RUnlock()
	require.Len(t, svc.groupUsageSummaryCache, 1)
	_, ok := svc.groupUsageSummaryCache[secondKey]
	require.True(t, ok)
}

func TestDashboardService_GroupUsageSummaryColdMissUsesSingleflight(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	var startedOnce sync.Once
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(ctx context.Context, _ time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	type result struct {
		value []usagestats.GroupUsageSummary
		err   error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	for range 8 {
		go func() {
			<-start
			value, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	<-started
	close(release)
	for range 8 {
		got := <-results
		require.NoError(t, got.err)
		require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, got.value)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
}

func TestDashboardService_CacheHitStale_TriggersAsyncRefresh(t *testing.T) {
	staleStats := &usagestats.DashboardStats{
		TotalUsers:     11,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	entry := dashboardStatsCacheEntry{
		Stats:     staleStats,
		UpdatedAt: time.Now().Add(-defaultDashboardStatsFreshTTL * 2).Unix(),
	}
	payload, err := json.Marshal(entry)
	require.NoError(t, err)

	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return string(payload), nil
		},
	}
	refreshCh := make(chan struct{}, 1)
	repo := &usageRepoStub{
		stats:  &usagestats.DashboardStats{TotalUsers: 22},
		onCall: refreshCh,
	}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, staleStats, got)

	select {
	case <-refreshCh:
	case <-time.After(1 * time.Second):
		t.Fatal("等待异步刷新超时")
	}
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&cache.setCalls) >= 1
	}, 1*time.Second, 10*time.Millisecond)
}

func TestDashboardService_CacheParseError_EvictsAndRefetches(t *testing.T) {
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "not-json", nil
		},
	}
	stats := &usagestats.DashboardStats{TotalUsers: 9}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.delCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
}

func TestDashboardService_CacheParseError_RepoFailure(t *testing.T) {
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "not-json", nil
		},
	}
	repo := &usageRepoStub{err: errors.New("db down")}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	_, err := svc.GetDashboardStats(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.delCalls))
}

func TestDashboardService_StatsUpdatedAtEpochWhenMissing(t *testing.T) {
	stats := &usagestats.DashboardStats{}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{Dashboard: config.DashboardCacheConfig{Enabled: false}}
	svc := NewDashboardService(repo, aggRepo, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, "1970-01-01T00:00:00Z", got.StatsUpdatedAt)
	require.True(t, got.StatsStale)
}

func TestDashboardService_StatsStaleFalseWhenFresh(t *testing.T) {
	aggNow := time.Now().UTC().Truncate(time.Second)
	stats := &usagestats.DashboardStats{}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: aggNow}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled:         true,
			IntervalSeconds: 60,
			LookbackSeconds: 120,
		},
	}
	svc := NewDashboardService(repo, aggRepo, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, aggNow.Format(time.RFC3339), got.StatsUpdatedAt)
	require.False(t, got.StatsStale)
}

func TestDashboardService_AggDisabled_UsesUsageLogsFallback(t *testing.T) {
	expected := &usagestats.DashboardStats{TotalUsers: 42}
	repo := &usageRepoStub{
		rangeStats: expected,
		err:        errors.New("should not call aggregated stats"),
	}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: false,
			Retention: config.DashboardAggregationRetentionConfig{
				UsageLogsDays: 7,
			},
		},
	}
	svc := NewDashboardService(repo, nil, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(42), got.TotalUsers)
	require.Equal(t, int32(0), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.rangeCalls))
	require.False(t, repo.rangeEnd.IsZero())
	require.Equal(t, truncateToDayUTC(repo.rangeEnd.AddDate(0, 0, -7)), repo.rangeStart)
}
