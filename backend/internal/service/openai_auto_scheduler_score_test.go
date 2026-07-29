package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoSchedulerScore_ErrorTriggersCircuit(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ConsecutiveErrorBreakerThreshold = 2
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	state = ApplyOpenAIAutoSchedulerEvent(now, state, OpenAIAutoSchedulerEventInput{
		EventType:  OpenAIAutoSchedulerEventError,
		StatusCode: ptrOpenAIAutoSchedulerInt(500),
		Message:    "upstream HTTP 500",
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateObserving, state.State)

	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Second), state, OpenAIAutoSchedulerEventInput{
		EventType:  OpenAIAutoSchedulerEventError,
		StatusCode: ptrOpenAIAutoSchedulerInt(502),
		Message:    "upstream HTTP 502",
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateOpen, state.State)
	require.NotNil(t, state.CooldownUntil)
	require.Less(t, state.FinalScore, 1000)
}

func TestOpenAIAutoSchedulerScore_SlowResponsesOnlyDegradeWeight(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.SlowThresholdMS = 10000
	settings.ConsecutiveSlowBreakerThreshold = 3
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	for i := 0; i < 3; i++ {
		state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Duration(i)*time.Second), state, OpenAIAutoSchedulerEventInput{
			EventType: OpenAIAutoSchedulerEventSlow,
			LatencyMS: ptrOpenAIAutoSchedulerInt(12000),
		}, settings)
	}
	require.Equal(t, OpenAIAutoSchedulerStateObserving, state.State)
	require.Nil(t, state.CooldownUntil)
	require.Less(t, state.FinalScore, 6000)

	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Minute), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventSuccess,
		LatencyMS: ptrOpenAIAutoSchedulerInt(1100),
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateRunning, state.State)
	require.Nil(t, state.CooldownUntil)
}

func TestOpenAIAutoSchedulerScore_TracksCountsAndRates(t *testing.T) {
	settings := enabledOpenAIAutoSchedulerSettings()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	state = ApplyOpenAIAutoSchedulerEvent(now, state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventSuccess,
		TtfbMS:    ptrOpenAIAutoSchedulerInt(300),
	}, settings)
	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Second), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventSlow,
		LatencyMS: ptrOpenAIAutoSchedulerInt(12000),
		TtfbMS:    ptrOpenAIAutoSchedulerInt(1000),
	}, settings)
	state = ApplyOpenAIAutoSchedulerEvent(now.Add(2*time.Second), state, OpenAIAutoSchedulerEventInput{
		EventType:  OpenAIAutoSchedulerEventError,
		StatusCode: ptrOpenAIAutoSchedulerInt(500),
	}, settings)
	state = ApplyOpenAIAutoSchedulerEvent(now.Add(3*time.Second), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventSevereSlow,
		LatencyMS: ptrOpenAIAutoSchedulerInt(25000),
	}, settings)

	require.Equal(t, int64(4), state.RequestCount)
	require.Equal(t, int64(2), state.TtfbSampleCount)
	require.InDelta(t, 0.5, state.SlowRate, 0.0001)
	require.InDelta(t, 0.25, state.ErrorRate, 0.0001)
	require.InDelta(t, 0.25, state.StuckRate, 0.0001)
}

func TestOpenAIAutoSchedulerScore_ManualResetBypassesDisabledSettings(t *testing.T) {
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")
	state.State = OpenAIAutoSchedulerStateOpen
	state.FinalScore = 500
	state.LatencyScore = -3500
	state.ErrorScore = -6000
	state.CooldownUntil = ptrOpenAIAutoSchedulerTime(time.Now().Add(time.Minute))

	next := ApplyOpenAIAutoSchedulerEvent(time.Now(), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventManualReset,
	}, DefaultOpenAIAutoSchedulerSettings())

	require.Equal(t, OpenAIAutoSchedulerStateRunning, next.State)
	require.Equal(t, 6000, next.FinalScore)
	require.Equal(t, 0, next.LatencyScore)
	require.Nil(t, next.CooldownUntil)
}

func ptrOpenAIAutoSchedulerInt(v int) *int {
	return &v
}

func ptrOpenAIAutoSchedulerTime(v time.Time) *time.Time {
	return &v
}
