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
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, int64(101), snapshot.AccountID)
	require.Equal(t, OpenAISchedulerTierPrimary, snapshot.Tier)
	require.Equal(t, "", snapshot.Reason)
	require.Equal(t, "primary", snapshot.DecisionReason)
	require.GreaterOrEqual(t, snapshot.HealthScore, 90.0)
	require.InDelta(t, 97.3, snapshot.HealthScore, 0.01)
	require.Nil(t, snapshot.CooldownUntil)
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
	require.Equal(t, OpenAISchedulerDegradeHighLatency, snapshot.Reason)
	require.Equal(t, "observe:high_latency", snapshot.DecisionReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestOpenAISchedulerHealthScore_ConsecutiveFailuresDegraded(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 10, 0, 0, time.UTC)
	snapshot := buildOpenAIAccountHealthSnapshot(103, openAIAccountHealthRuntime{
		successEWMA:         0.20,
		errorEWMA:           0.80,
		ttftEWMA:            0,
		consecutiveSuccess:  0,
		consecutiveFailures: 3,
		lastDegradeReason:   OpenAISchedulerDegradeTimeout,
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, OpenAISchedulerTierDegraded, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeTimeout, snapshot.Reason)
	require.Equal(t, "degraded:timeout", snapshot.DecisionReason)
	require.NotNil(t, snapshot.CooldownUntil)
	require.True(t, snapshot.CooldownUntil.After(now))
}

func TestOpenAISchedulerHealthScore_CooldownExpiredObserve(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 20, 0, 0, time.UTC)
	expired := now.Add(-1 * time.Minute)
	snapshot := buildOpenAIAccountHealthSnapshot(104, openAIAccountHealthRuntime{
		successEWMA:         0.88,
		errorEWMA:           0.12,
		ttftEWMA:            700,
		consecutiveSuccess:  2,
		consecutiveFailures: 0,
		lastDegradeReason:   OpenAISchedulerDegradeManual,
		cooldownUntilUnix:   expired.Unix(),
	}, defaultOpenAISchedulerHealthSettings(), now)

	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeRecovering, snapshot.Reason)
	require.Equal(t, "observe:recovering", snapshot.DecisionReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestDefaultOpenAIAccountScheduler_ReportResultUpdatesHealth(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	ttft := 180
	scheduler.ReportResult(2001, true, &ttft)

	snapshot, found := scheduler.SnapshotAccountHealth(context.Background(), 2001)
	require.True(t, found)
	require.Equal(t, int64(2001), snapshot.AccountID)
	require.Greater(t, snapshot.SuccessEWMA, 0.0)
	require.GreaterOrEqual(t, snapshot.HealthScore, 0.0)
	require.LessOrEqual(t, snapshot.HealthScore, 100.0)
	require.InDelta(t, 180.0, snapshot.TTFTEWMA, 0.01)
}
