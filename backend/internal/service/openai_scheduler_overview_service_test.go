package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAISchedulerOverviewRepoStub struct {
	metrics      OpenAISchedulerOverviewMetrics
	healthItems  []OpenAISchedulerHealthRecord
	healthTotal  int64
	overviewErr  error
	healthErr    error
	overviewCall OpenAISchedulerOverviewParams
	healthCall   OpenAISchedulerHealthParams
	actualItems  []OpenAISchedulerRankingActual
	actualCall   OpenAISchedulerRankingActualParams
	actualErr    error
}

func (s *openAISchedulerOverviewRepoStub) ListOpenAISchedulerRankingActual(_ context.Context, params OpenAISchedulerRankingActualParams) ([]OpenAISchedulerRankingActual, error) {
	s.actualCall = params
	return s.actualItems, s.actualErr
}

func (s *openAISchedulerOverviewRepoStub) GetOpenAISchedulerOverviewMetrics(_ context.Context, params OpenAISchedulerOverviewParams) (OpenAISchedulerOverviewMetrics, error) {
	s.overviewCall = params
	return s.metrics, s.overviewErr
}

func (s *openAISchedulerOverviewRepoStub) ListOpenAISchedulerHealth(_ context.Context, params OpenAISchedulerHealthParams) ([]OpenAISchedulerHealthRecord, int64, error) {
	s.healthCall = params
	return s.healthItems, s.healthTotal, s.healthErr
}

type openAISchedulerOverviewLoadStub struct {
	calls    int
	accounts []AccountWithConcurrency
	loads    map[int64]*AccountLoadInfo
	err      error
}

func (s *openAISchedulerOverviewLoadStub) GetAccountsLoadBatch(_ context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	s.calls++
	s.accounts = append([]AccountWithConcurrency(nil), accounts...)
	return s.loads, s.err
}

type openAISchedulerOverviewSettingsStub struct {
	settings      OpenAIAutoSchedulerSettings
	engineEnabled bool
}

func (s openAISchedulerOverviewSettingsStub) IsOpenAIAdvancedSchedulerEnabled(context.Context) bool {
	return s.engineEnabled
}

func (s openAISchedulerOverviewSettingsStub) GetOpenAIAutoSchedulerSettings(context.Context) OpenAIAutoSchedulerSettings {
	return s.settings
}

func TestOpenAISchedulerOverviewServiceBuildsControlConsoleMetrics(t *testing.T) {
	repo := &openAISchedulerOverviewRepoStub{metrics: OpenAISchedulerOverviewMetrics{
		E2EP50MS:       2970,
		E2EP90MS:       7210,
		SelectionP95MS: 18,
		ProbeRatio:     0.24,
		Groups:         []OpenAISchedulerGroupSummary{{ID: 33, Name: "Codex", Enabled: true, AccountCount: 4, E2EP90MS: 7210}},
	}}
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.now = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	got, err := svc.GetOverview(context.Background(), OpenAISchedulerOverviewParams{GroupID: 33, Window: 6 * time.Hour})

	require.NoError(t, err)
	require.Equal(t, 2970.0, got.E2EP50MS)
	require.Equal(t, 7210.0, got.E2EP90MS)
	require.Equal(t, 18.0, got.SelectionP95MS)
	require.InDelta(t, 0.24, got.ProbeRatio, 0.0001)
	require.Equal(t, time.Hour, repo.overviewCall.Bucket)
	require.Equal(t, 6*time.Hour, repo.overviewCall.Window)
	require.Equal(t, 33, int(repo.overviewCall.GroupID))
	require.Equal(t, "ok", got.Groups[0].AlertLevel)
	require.Equal(t, svc.now(), repo.overviewCall.EndTime)
	require.Equal(t, svc.now().Add(-6*time.Hour), repo.overviewCall.StartTime)
}

func TestOpenAISchedulerOverviewServiceUsesBoundedBuckets(t *testing.T) {
	tests := []struct {
		name       string
		window     time.Duration
		wantBucket time.Duration
		wantErr    bool
	}{
		{name: "one hour", window: time.Hour, wantBucket: time.Hour},
		{name: "six hours", window: 6 * time.Hour, wantBucket: time.Hour},
		{name: "one day", window: 24 * time.Hour, wantBucket: time.Hour},
		{name: "seven days", window: 7 * 24 * time.Hour, wantBucket: 6 * time.Hour},
		{name: "unsupported", window: 2 * time.Hour, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &openAISchedulerOverviewRepoStub{}
			svc := NewOpenAISchedulerOverviewService(repo)
			_, err := svc.GetOverview(context.Background(), OpenAISchedulerOverviewParams{Window: tt.window})
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, time.Duration(0), repo.overviewCall.Window)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantBucket, repo.overviewCall.Bucket)
		})
	}
}

func TestOpenAISchedulerOverviewServiceMapsPaginatedHealthWithOneLoadBatch(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	price := 0.75
	repo := &openAISchedulerOverviewRepoStub{
		healthTotal: 3,
		healthItems: []OpenAISchedulerHealthRecord{
			{AccountID: 10, AccountName: "primary", GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 1200, RealSampleCount: 20, ProbeSampleCount: 2, ErrorRate: 0.01, RateLimitedRate: 0.02, ServerErrorRate: 0.03, LoadCapacity: 4, ChannelPrice: &price, UpdatedAt: now.Add(-2 * time.Second), ExpiresAt: now.Add(time.Minute)},
			{AccountID: 10, AccountName: "primary", GroupID: 82, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateOpen, PredictedTTFTMS: 9200, LoadCapacity: 4, UpdatedAt: now.Add(-3 * time.Second), ExpiresAt: now.Add(time.Minute)},
		},
	}
	loads := &openAISchedulerOverviewLoadStub{loads: map[int64]*AccountLoadInfo{10: {AccountID: 10, CurrentConcurrency: 2, WaitingCount: 1}}}
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Mode = OpenAIAutoSchedulerModeBalanced
	settings.ShadowMode = true
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.loads = loads
	svc.settings = openAISchedulerOverviewSettingsStub{settings: settings}
	svc.now = func() time.Time { return now }

	got, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{GroupID: 33, State: "running", Page: 2, PageSize: 20})

	require.NoError(t, err)
	require.Equal(t, int64(3), got.Total)
	require.Len(t, got.Items, 2)
	require.Equal(t, 1, loads.calls)
	require.Equal(t, []AccountWithConcurrency{{ID: 10, MaxConcurrency: 4}}, loads.accounts)
	require.Equal(t, 2, got.Items[0].LoadInflight)
	require.Equal(t, 4, got.Items[0].LoadCapacity)
	require.Equal(t, 1, got.Items[0].WaitingCount)
	require.Equal(t, "context_required", got.Items[0].Decision)
	require.Equal(t, "request_context_required", got.Items[0].DecisionReason)
	require.Equal(t, "circuit_rejected", got.Items[1].Decision)
	require.Equal(t, "open", got.Items[1].DecisionReason)
	require.Equal(t, OpenAIAutoSchedulerModeBalanced, got.Items[0].SchedulerMode)
	require.True(t, got.Items[0].ShadowMode)
	require.Equal(t, int64(2000), *got.Items[0].SnapshotAgeMS)
	require.Nil(t, got.Items[0].StickyEscapeReason)
	require.Equal(t, int64(33), repo.healthCall.GroupID)
}

func TestOpenAISchedulerOverviewServiceClassifiesHealthWithoutInventingRequestDecisions(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	blockedUntil := now.Add(time.Minute)
	repo := &openAISchedulerOverviewRepoStub{healthItems: []OpenAISchedulerHealthRecord{
		{AccountID: 1, GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 2501, ExpiresAt: now.Add(time.Minute)},
		{AccountID: 2, GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 1000, ExpiresAt: now.Add(-time.Second)},
		{AccountID: 3, GroupID: 33},
		{AccountID: 4, GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: "disabled", Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, ExpiresAt: now.Add(time.Minute)},
		{AccountID: 5, GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: false, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, ExpiresAt: now.Add(time.Minute)},
		{AccountID: 6, GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, TempUnschedulableUntil: &blockedUntil, TempUnschedulableReason: "price guard", ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse", State: OpenAIAutoSchedulerStateRunning, ExpiresAt: now.Add(time.Minute)},
	}}
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.now = func() time.Time { return now }

	got, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{})

	require.NoError(t, err)
	require.Equal(t, "context_required", got.Items[0].Decision)
	require.Equal(t, "request_context_required", got.Items[0].DecisionReason)
	require.Equal(t, "stale", got.Items[1].Decision)
	require.Equal(t, "snapshot_expired", got.Items[1].DecisionReason)
	require.Equal(t, "health_unavailable", got.Items[2].Decision)
	require.Equal(t, "snapshot_missing", got.Items[2].DecisionReason)
	require.Equal(t, "hard_filtered", got.Items[3].Decision)
	require.Equal(t, "account_inactive", got.Items[3].DecisionReason)
	require.Equal(t, "hard_filtered", got.Items[4].Decision)
	require.Equal(t, "account_unschedulable", got.Items[4].DecisionReason)
	require.Equal(t, "hard_filtered", got.Items[5].Decision)
	require.Equal(t, "temporarily_blocked: price guard", got.Items[5].DecisionReason)
	for _, item := range got.Items {
		require.Nil(t, item.StickyEscapeReason)
	}
}

func TestClassifyOpenAISchedulerHealthRecordKnownHardFilters(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Minute)
	base := OpenAISchedulerHealthRecord{
		GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true,
		AccountStatus: StatusActive, Schedulable: true,
		ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse",
		State: OpenAIAutoSchedulerStateRunning, ExpiresAt: future,
	}
	tests := []struct {
		name       string
		mutate     func(*OpenAISchedulerHealthRecord)
		want       string
		wantReason string
	}{
		{name: "group inactive", mutate: func(r *OpenAISchedulerHealthRecord) { r.GroupStatus = StatusDisabled }, want: "hard_filtered", wantReason: "group_inactive"},
		{name: "group scheduler disabled", mutate: func(r *OpenAISchedulerHealthRecord) { r.GroupAutoSchedulerEnabled = false }, want: "hard_filtered", wantReason: "group_scheduler_disabled"},
		{name: "expired auto pause", mutate: func(r *OpenAISchedulerHealthRecord) { r.AutoPauseOnExpired = true; r.AccountExpiresAt = &past }, want: "hard_filtered", wantReason: "account_expired"},
		{name: "expired without auto pause needs context", mutate: func(r *OpenAISchedulerHealthRecord) { r.AutoPauseOnExpired = false; r.AccountExpiresAt = &past }, want: "context_required", wantReason: "request_context_required"},
		{name: "overloaded", mutate: func(r *OpenAISchedulerHealthRecord) { r.OverloadUntil = &future }, want: "hard_filtered", wantReason: "account_overloaded"},
		{name: "past overload needs context", mutate: func(r *OpenAISchedulerHealthRecord) { r.OverloadUntil = &past }, want: "context_required", wantReason: "request_context_required"},
		{name: "rate limited", mutate: func(r *OpenAISchedulerHealthRecord) { r.RateLimitResetAt = &future }, want: "hard_filtered", wantReason: "account_rate_limited"},
		{name: "past rate limit needs context", mutate: func(r *OpenAISchedulerHealthRecord) { r.RateLimitResetAt = &past }, want: "context_required", wantReason: "request_context_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := base
			tt.mutate(&record)
			got, reason := classifyOpenAISchedulerHealthRecord(record, now)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestOpenAISchedulerOverviewServiceUsesEffectiveLoadCapacityForBatchLoad(t *testing.T) {
	repo := &openAISchedulerOverviewRepoStub{healthItems: []OpenAISchedulerHealthRecord{
		{AccountID: 20, LoadCapacity: 20},
		{AccountID: 5, LoadCapacity: 5},
		{AccountID: 1, LoadCapacity: 1},
	}}
	loads := &openAISchedulerOverviewLoadStub{}
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.loads = loads

	_, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{})

	require.NoError(t, err)
	require.Equal(t, []AccountWithConcurrency{
		{ID: 20, MaxConcurrency: 20},
		{ID: 5, MaxConcurrency: 5},
		{ID: 1, MaxConcurrency: 1},
	}, loads.accounts)
}

func TestOpenAISchedulerOverviewServiceHealthEmptyAndErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		repo := &openAISchedulerOverviewRepoStub{}
		loads := &openAISchedulerOverviewLoadStub{}
		svc := NewOpenAISchedulerOverviewService(repo)
		svc.loads = loads
		got, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{Page: 1, PageSize: 20})
		require.NoError(t, err)
		require.Empty(t, got.Items)
		require.Zero(t, loads.calls)
	})

	t.Run("repository error", func(t *testing.T) {
		wantErr := errors.New("health query failed")
		svc := NewOpenAISchedulerOverviewService(&openAISchedulerOverviewRepoStub{healthErr: wantErr})
		_, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{})
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("load error", func(t *testing.T) {
		wantErr := errors.New("load query failed")
		svc := NewOpenAISchedulerOverviewService(&openAISchedulerOverviewRepoStub{healthItems: []OpenAISchedulerHealthRecord{{AccountID: 1, LoadCapacity: 2}}})
		svc.loads = &openAISchedulerOverviewLoadStub{err: wantErr}
		_, err := svc.ListHealth(context.Background(), OpenAISchedulerHealthParams{})
		require.ErrorIs(t, err, wantErr)
	})
}

func TestOpenAISchedulerOverviewServiceBuildsRankingFromSharedPolicyAndActualTraffic(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	fastPrice, cheapPrice := 0.8, 0.3
	partition := OpenAISchedulerRankingPartition{GroupID: 33, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse"}
	repo := &openAISchedulerOverviewRepoStub{
		healthItems: []OpenAISchedulerHealthRecord{
			{AccountID: 10, AccountName: "fast", GroupID: 33, GroupPriority: 1, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 700, RealSampleCount: 50, ErrorRate: 0.01, LoadCapacity: 10, ChannelPrice: &fastPrice, UpdatedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute)},
			{AccountID: 20, AccountName: "cheap", GroupID: 33, GroupPriority: 2, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true, AccountStatus: StatusActive, Schedulable: true, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 1100, RealSampleCount: 30, ErrorRate: 0.02, LoadCapacity: 10, ChannelPrice: &cheapPrice, UpdatedAt: now.Add(-2 * time.Second), ExpiresAt: now.Add(time.Minute)},
		},
		actualItems: []OpenAISchedulerRankingActual{
			{Key: OpenAISchedulerRankingActualKey{AccountID: 10, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport}, RequestCount: 80, TTFTP50MS: 750, TTFTP90MS: 1200, EstimatedCost: 64},
			{Key: OpenAISchedulerRankingActualKey{AccountID: 20, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport}, RequestCount: 20, TTFTP50MS: 1180, TTFTP90MS: 1900, EstimatedCost: 6},
		},
	}
	loads := &openAISchedulerOverviewLoadStub{loads: map[int64]*AccountLoadInfo{
		10: {AccountID: 10, CurrentConcurrency: 2, LoadRate: 20},
		20: {AccountID: 20, CurrentConcurrency: 1, LoadRate: 10},
	}}
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.Mode = OpenAIAutoSchedulerModePerformance
	settings.ShadowMode = false
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.loads = loads
	svc.settings = openAISchedulerOverviewSettingsStub{settings: settings, engineEnabled: true}
	svc.now = func() time.Time { return now }

	got, err := svc.ListRankings(context.Background(), OpenAISchedulerRankingParams{GroupID: 33, Window: time.Hour, Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Equal(t, OpenAIAutoSchedulerModePerformance, got.PolicyContext.EffectiveMode)
	require.True(t, got.PolicyContext.EngineEnabled)
	require.Equal(t, int64(100), got.Summary.RequestCount)
	require.Equal(t, 2, got.Summary.EligibleCount)
	require.Len(t, got.Items, 2)
	require.Equal(t, 1, got.Items[0].Rank)
	require.Equal(t, int64(10), got.Items[0].AccountID)
	require.InDelta(t, 0.8, got.Items[0].ActualShare, 0.0001)
	require.Greater(t, got.Items[0].TargetShare, got.Items[1].TargetShare)
	require.InDelta(t, 1, got.Items[0].TargetShare+got.Items[1].TargetShare, 0.0001)
	require.Equal(t, now.Add(-time.Hour), repo.actualCall.StartTime)
	require.Equal(t, now, repo.actualCall.EndTime)
}

func TestOpenAISchedulerOverviewServiceAggregatesRankingByPhysicalAccountAcrossModels(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	price := 0.4
	base := OpenAISchedulerHealthRecord{
		GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true,
		AccountStatus: StatusActive, Schedulable: true, Endpoint: "responses", Transport: "http_sse",
		State: OpenAIAutoSchedulerStateRunning, RealSampleCount: 20, LoadCapacity: 10,
		ChannelPrice: &price, UpdatedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute),
	}
	health := func(accountID int64, name, model string, ttft float64) OpenAISchedulerHealthRecord {
		record := base
		record.AccountID = accountID
		record.AccountName = name
		record.ModelFamily = model
		record.PredictedTTFTMS = ttft
		return record
	}
	actual := func(accountID int64, model string, requests int64, p50 float64) OpenAISchedulerRankingActual {
		return OpenAISchedulerRankingActual{
			Key: OpenAISchedulerRankingActualKey{
				AccountID: accountID, ModelFamily: model, Endpoint: "responses", Transport: "http_sse",
			},
			RequestCount: requests, TTFTP50MS: p50, TTFTP90MS: p50 * 2, EstimatedCost: float64(requests) * price,
		}
	}
	repo := &openAISchedulerOverviewRepoStub{
		healthItems: []OpenAISchedulerHealthRecord{
			health(10, "account-a", "codex-auto-review", 800),
			health(20, "account-b", "codex-auto-review", 1000),
			health(10, "account-a", "gpt-5.4", 900),
			health(20, "account-b", "gpt-5.4", 700),
		},
		actualItems: []OpenAISchedulerRankingActual{
			actual(10, "codex-auto-review", 80, 850),
			actual(20, "codex-auto-review", 20, 1050),
			actual(10, "gpt-5.4", 10, 950),
			actual(20, "gpt-5.4", 90, 750),
		},
	}
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ShadowMode = false
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.settings = openAISchedulerOverviewSettingsStub{settings: settings, engineEnabled: true}
	svc.now = func() time.Time { return now }

	got, err := svc.ListRankings(context.Background(), OpenAISchedulerRankingParams{
		GroupID: 33, Window: time.Hour, Page: 1, PageSize: 20,
	})

	require.NoError(t, err)
	require.Equal(t, 2, got.Summary.CandidateCount)
	require.Equal(t, int64(200), got.Summary.RequestCount)
	require.Equal(t, int64(2), got.Total)
	require.Len(t, got.Items, 2)
	byAccountID := make(map[int64]OpenAISchedulerRankingItem, len(got.Items))
	for _, item := range got.Items {
		byAccountID[item.AccountID] = item
	}
	require.Len(t, byAccountID, 2)
	require.Equal(t, 2, byAccountID[10].PartitionCount)
	require.Equal(t, 2, byAccountID[20].PartitionCount)
	require.Empty(t, byAccountID[10].Partition.ModelFamily)
	require.Equal(t, int64(90), byAccountID[10].SelectedRequests)
	require.Equal(t, int64(110), byAccountID[20].SelectedRequests)
	require.InDelta(t, 0.45, byAccountID[10].ActualShare, 0.0001)
	require.InDelta(t, 0.55, byAccountID[20].ActualShare, 0.0001)
	require.InDelta(t, 1, byAccountID[10].TargetShare+byAccountID[20].TargetShare, 0.0001)
}

func TestOpenAISchedulerOverviewServiceMarksLegacyFallbackWhenEngineIsDisabled(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	repo := &openAISchedulerOverviewRepoStub{healthItems: []OpenAISchedulerHealthRecord{{
		AccountID: 10, AccountName: "primary", GroupID: 33, GroupStatus: StatusActive, GroupAutoSchedulerEnabled: true,
		AccountStatus: StatusActive, Schedulable: true, ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse",
		State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 800, RealSampleCount: 20, UpdatedAt: now, ExpiresAt: now.Add(time.Minute),
	}}}
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ShadowMode = false
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.settings = openAISchedulerOverviewSettingsStub{settings: settings, engineEnabled: false}
	svc.now = func() time.Time { return now }

	got, err := svc.ListRankings(context.Background(), OpenAISchedulerRankingParams{GroupID: 33, Window: time.Hour})

	require.NoError(t, err)
	require.Equal(t, OpenAIAutoSchedulerModeLegacy, got.PolicyContext.EffectiveMode)
	require.Equal(t, "engine_disabled", got.PolicyContext.FallbackReason)
	require.Contains(t, got.Items[0].DeviationReasons, "legacy_fallback")
}
