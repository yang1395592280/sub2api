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

func TestOpenAIAutoSchedulerScore_SlowResponsesDegradeThenRecover(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.SlowThresholdMS = 10000
	settings.ConsecutiveSlowBreakerThreshold = 3
	settings.HalfOpenSuccessThreshold = 2
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	for i := 0; i < 3; i++ {
		state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Duration(i)*time.Second), state, OpenAIAutoSchedulerEventInput{
			EventType: OpenAIAutoSchedulerEventSlow,
			LatencyMS: ptrOpenAIAutoSchedulerInt(12000),
		}, settings)
	}
	require.Equal(t, OpenAIAutoSchedulerStateOpen, state.State)

	state.CooldownUntil = ptrOpenAIAutoSchedulerTime(now.Add(-time.Second))
	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Minute), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventProbeSuccess,
		LatencyMS: ptrOpenAIAutoSchedulerInt(1100),
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateHalfOpen, state.State)

	state = ApplyOpenAIAutoSchedulerEvent(now.Add(2*time.Minute), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventProbeSuccess,
		LatencyMS: ptrOpenAIAutoSchedulerInt(900),
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateRunning, state.State)
	require.Greater(t, state.FinalScore, 6000)
}

func ptrOpenAIAutoSchedulerInt(v int) *int {
	return &v
}

func ptrOpenAIAutoSchedulerTime(v time.Time) *time.Time {
	return &v
}
