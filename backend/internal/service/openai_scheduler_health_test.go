package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerHealthSettings_Defaults(t *testing.T) {
	settings := defaultOpenAISchedulerHealthSettings()

	require.False(t, settings.HealthRankingEnabled)
	require.Equal(t, 0.30, settings.PrimaryRatio)
	require.Equal(t, 1, settings.PrimaryMinCount)
	require.Equal(t, 2500, settings.TTFTDegradeMS)
	require.Equal(t, 0.35, settings.ErrorRateDegradeThreshold)
	require.Equal(t, 3, settings.ConsecutiveFailureThreshold)
	require.Equal(t, 5, settings.RecoverSuccessThreshold)
	require.Equal(t, 10*time.Minute, settings.Cooldown)
	require.Equal(t, 0.0, settings.ObserveProbeRatio)
}

func TestOpenAISchedulerHealthScore_SuccessLowLatencyPrimary(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	snapshot := buildOpenAIAccountHealthSnapshot(101, openAIAccountHealthRuntime{
		successEWMA:         0.95,
		errorEWMA:           0.05,
		ttftEWMA:            420,
		consecutiveSuccess:  2,
		consecutiveFailures: 0,
		lastSelectedUnixSec: now.Add(-2 * time.Minute).Unix(),
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, int64(101), snapshot.AccountID)
	require.Equal(t, OpenAISchedulerTierPrimary, snapshot.Tier)
	require.Equal(t, "", snapshot.DegradeReason)
	require.Equal(t, "health score is high and account is eligible for primary routing", snapshot.DecisionReason)
	require.GreaterOrEqual(t, snapshot.HealthScore, 90.0)
	require.InDelta(t, 97.3, snapshot.HealthScore, 0.01)
	require.Nil(t, snapshot.CooldownUntil)
	require.NotNil(t, snapshot.LastSelectedAt)
	require.Equal(t, now.Add(-2*time.Minute).Unix(), snapshot.LastSelectedAt.Unix())
}

func TestOpenAISchedulerHealthScore_HighLatencyObserve(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 5, 0, 0, time.UTC)
	snapshot := buildOpenAIAccountHealthSnapshot(102, openAIAccountHealthRuntime{
		successEWMA:         0.98,
		errorEWMA:           0.02,
		ttftEWMA:            3200,
		consecutiveSuccess:  1,
		consecutiveFailures: 0,
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeHighLatency, snapshot.DegradeReason)
	require.Equal(t, "account is being observed after high_latency", snapshot.DecisionReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestOpenAISchedulerHealthScore_ConsecutiveFailuresDegraded(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)
	cooldownUntil := now.Add(8 * time.Minute)
	snapshot := buildOpenAIAccountHealthSnapshot(103, openAIAccountHealthRuntime{
		successEWMA:          0.20,
		errorEWMA:            0.80,
		ttftEWMA:             0,
		consecutiveSuccess:   0,
		consecutiveFailures:  3,
		lastDegradeReason:    OpenAISchedulerDegradeTimeout,
		cooldownUntilUnixSec: cooldownUntil.Unix(),
		lastErrorUnixSec:     now.Add(-30 * time.Second).Unix(),
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, OpenAISchedulerTierDegraded, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeTimeout, snapshot.DegradeReason)
	require.Equal(t, "account is degraded because of timeout", snapshot.DecisionReason)
	require.NotNil(t, snapshot.CooldownUntil)
	require.Equal(t, cooldownUntil.Unix(), snapshot.CooldownUntil.Unix())
	require.NotNil(t, snapshot.LastErrorAt)
	require.Equal(t, now.Add(-30*time.Second).Unix(), snapshot.LastErrorAt.Unix())
}

func TestOpenAISchedulerHealthScore_CooldownExpiredObserve(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 20, 0, 0, time.UTC)
	expired := now.Add(-1 * time.Minute)
	snapshot := buildOpenAIAccountHealthSnapshot(104, openAIAccountHealthRuntime{
		successEWMA:          0.88,
		errorEWMA:            0.12,
		ttftEWMA:             700,
		consecutiveSuccess:   2,
		consecutiveFailures:  0,
		lastDegradeReason:    OpenAISchedulerDegradeManual,
		cooldownUntilUnixSec: expired.Unix(),
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeRecovering, snapshot.DegradeReason)
	require.Equal(t, "account is being observed after recovering", snapshot.DecisionReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestDefaultOpenAIAccountScheduler_ReportResultUpdatesHealth(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	scheduler.ReportResult(2001, false, nil)
	ttft := 180
	scheduler.ReportResult(2001, true, &ttft)

	snapshot, found := scheduler.SnapshotAccountHealth(context.Background(), 2001)
	require.True(t, found)
	require.Equal(t, int64(2001), snapshot.AccountID)
	require.Greater(t, snapshot.SuccessRateEWMA, 0.0)
	require.Greater(t, snapshot.ErrorRateEWMA, 0.0)
	require.GreaterOrEqual(t, snapshot.HealthScore, 0.0)
	require.LessOrEqual(t, snapshot.HealthScore, 100.0)
	require.Greater(t, snapshot.TTFTEWMAMS, 0.0)
	require.InDelta(t, 180.0, snapshot.TTFTEWMAMS, 0.01)
	require.NotNil(t, snapshot.LastSelectedAt)
	require.NotNil(t, snapshot.LastErrorAt)
}

func TestDefaultOpenAIAccountScheduler_UpdateHealthSettingsClampsValues(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	scheduler.UpdateHealthSettings(OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        true,
		PrimaryRatio:                2,
		PrimaryMinCount:             -4,
		TTFTDegradeMS:               -1,
		ErrorRateDegradeThreshold:   9,
		ConsecutiveFailureThreshold: -2,
		RecoverSuccessThreshold:     -5,
		Cooldown:                    -time.Second,
		ObserveProbeRatio:           3,
	})

	settings := scheduler.SnapshotHealthSettings()
	defaults := defaultOpenAISchedulerHealthSettings()
	require.True(t, settings.HealthRankingEnabled)
	require.Equal(t, defaults.PrimaryRatio, settings.PrimaryRatio)
	require.Equal(t, defaults.PrimaryMinCount, settings.PrimaryMinCount)
	require.Equal(t, defaults.TTFTDegradeMS, settings.TTFTDegradeMS)
	require.Equal(t, defaults.ErrorRateDegradeThreshold, settings.ErrorRateDegradeThreshold)
	require.Equal(t, defaults.ConsecutiveFailureThreshold, settings.ConsecutiveFailureThreshold)
	require.Equal(t, defaults.RecoverSuccessThreshold, settings.RecoverSuccessThreshold)
	require.Equal(t, defaults.Cooldown, settings.Cooldown)
	require.Equal(t, defaults.ObserveProbeRatio, settings.ObserveProbeRatio)
}

func TestDefaultOpenAIAccountScheduler_ManualCooldownAndClear(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	err := scheduler.ApplyHealthAction(3001, OpenAISchedulerHealthAction{
		Action:   "cooldown",
		Reason:   "manual verification",
		Duration: time.Minute,
	})
	require.NoError(t, err)

	snapshot, found := scheduler.SnapshotAccountHealth(context.Background(), 3001)
	require.True(t, found)
	require.Equal(t, OpenAISchedulerTierDegraded, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeManual, snapshot.DegradeReason)
	require.NotNil(t, snapshot.CooldownUntil)

	err = scheduler.ApplyHealthAction(3001, OpenAISchedulerHealthAction{Action: "clear_cooldown"})
	require.NoError(t, err)

	snapshot, found = scheduler.SnapshotAccountHealth(context.Background(), 3001)
	require.True(t, found)
	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeRecovering, snapshot.DegradeReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestDefaultOpenAIAccountScheduler_InvalidManualAction(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	err := scheduler.ApplyHealthAction(3002, OpenAISchedulerHealthAction{Action: "pin_primary"})

	require.Error(t, err)
}

func TestOpenAIGatewayService_HealthSchedulerDisabledReturnsDefaults(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc := &OpenAIGatewayService{}

	settings := svc.SnapshotOpenAISchedulerHealthSettings()
	snapshot, ok := svc.SnapshotOpenAIAccountHealth(context.Background(), 1)

	require.False(t, settings.HealthRankingEnabled)
	require.False(t, ok)
	require.Equal(t, OpenAIAccountHealthSnapshot{}, snapshot)
}

func TestOpenAISchedulerHealthSettingsRoundTrip(t *testing.T) {
	repo := &openAISchedulerSettingRepoStub{values: map[string]string{}}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{
			settingService: &SettingService{settingRepo: repo},
		},
	}

	input := OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        true,
		PrimaryRatio:                0.4,
		PrimaryMinCount:             2,
		TTFTDegradeMS:               1800,
		ErrorRateDegradeThreshold:   0.25,
		ConsecutiveFailureThreshold: 2,
		RecoverSuccessThreshold:     4,
		Cooldown:                    3 * time.Minute,
		ObserveProbeRatio:           0.05,
	}

	require.NoError(t, svc.SaveOpenAISchedulerHealthSettings(context.Background(), input))
	got := svc.SnapshotOpenAISchedulerHealthSettings()

	require.True(t, got.HealthRankingEnabled)
	require.Equal(t, 0.4, got.PrimaryRatio)
	require.Equal(t, 2, got.PrimaryMinCount)
	require.Equal(t, 1800, got.TTFTDegradeMS)
	require.Equal(t, 0.25, got.ErrorRateDegradeThreshold)
	require.Equal(t, 2, got.ConsecutiveFailureThreshold)
	require.Equal(t, 4, got.RecoverSuccessThreshold)
	require.Equal(t, 3*time.Minute, got.Cooldown)
	require.Equal(t, 0.05, got.ObserveProbeRatio)
}

func TestOpenAISchedulerHealthSettings_EnableHealthRankingTurnsOnAdvancedScheduler(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	repo := &openAISchedulerSettingRepoStub{values: map[string]string{}}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{
			settingService: &SettingService{settingRepo: repo},
		},
	}
	require.False(t, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))

	require.NoError(t, svc.SaveOpenAISchedulerHealthSettings(context.Background(), OpenAISchedulerHealthSettings{
		HealthRankingEnabled: true,
	}))

	require.Equal(t, "true", repo.values[openAIAdvancedSchedulerSettingKey])
	require.True(t, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))
	require.NoError(t, svc.ApplyOpenAISchedulerHealthAction(11854, OpenAISchedulerHealthAction{Action: "promote_observe"}))
}

func TestOpenAISchedulerHealthSettings_LegacyHealthRankingEnablesAdvancedScheduler(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	repo := &openAISchedulerSettingRepoStub{
		values: map[string]string{
			openAISchedulerHealthRankingEnabledKey: "true",
		},
	}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{
			settingService: &SettingService{settingRepo: repo},
		},
	}

	require.True(t, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))
	require.NoError(t, svc.ApplyOpenAISchedulerHealthAction(11854, OpenAISchedulerHealthAction{Action: "promote_observe"}))
}

func TestOpenAIGatewayService_ListAllOpenAISchedulerAccountSnapshotsIncludesGroupedAccounts(t *testing.T) {
	groupID := int64(33)
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{
			schedulerTestOpenAIAccountRepo: schedulerTestOpenAIAccountRepo{
				accounts: []Account{
					{
						ID:            11854,
						Name:          "grouped-openai",
						Platform:      PlatformOpenAI,
						Type:          AccountTypeOAuth,
						Status:        StatusActive,
						Schedulable:   true,
						GroupIDs:      []int64{groupID},
						AccountGroups: []AccountGroup{{GroupID: groupID}},
					},
					{
						ID:          11857,
						Name:        "ungrouped-openai",
						Platform:    PlatformOpenAI,
						Type:        AccountTypeOAuth,
						Status:      StatusActive,
						Schedulable: true,
					},
				},
			},
		},
	}

	allItems, err := svc.ListAllOpenAISchedulerAccountSnapshots(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{11854, 11857}, openAISchedulerSnapshotIDs(allItems))

	runtimeItems, err := svc.ListOpenAISchedulerAccountSnapshots(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, []int64{11857}, openAISchedulerSnapshotIDs(runtimeItems))
}

func openAISchedulerSnapshotIDs(items []OpenAISchedulerAccountSnapshot) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.AccountID)
	}
	return ids
}

type openAISchedulerSettingRepoStub struct {
	values map[string]string
}

func (r *openAISchedulerSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *openAISchedulerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (r *openAISchedulerSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *openAISchedulerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *openAISchedulerSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *openAISchedulerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *openAISchedulerSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}
