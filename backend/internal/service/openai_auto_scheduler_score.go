package service

import (
	"strings"
	"time"
)

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 10000 {
		return 10000
	}
	return v
}

func NewOpenAIAutoSchedulerScoreState(accountID, groupID int64, model string) OpenAIAutoSchedulerScoreState {
	return OpenAIAutoSchedulerScoreState{
		AccountID:  accountID,
		GroupID:    groupID,
		Model:      strings.TrimSpace(model),
		BaseScore:  6000,
		FinalScore: 6000,
		State:      OpenAIAutoSchedulerStateRunning,
	}
}

func ApplyOpenAIAutoSchedulerEvent(now time.Time, state OpenAIAutoSchedulerScoreState, input OpenAIAutoSchedulerEventInput, settings OpenAIAutoSchedulerSettings) OpenAIAutoSchedulerScoreState {
	if !input.OccurredAt.IsZero() {
		now = input.OccurredAt
	}
	settings = normalizeOpenAIAutoSchedulerSettings(settings)
	state.State = normalizeOpenAIAutoSchedulerState(state.State)
	state.LastCheckedAt = openAIAutoSchedulerTimePtr(now)
	state.LastLatencyMS = copyOpenAIAutoSchedulerIntPtr(input.LatencyMS)
	state.LastTtfbMS = copyOpenAIAutoSchedulerIntPtr(input.TtfbMS)
	state.LastStatusCode = copyOpenAIAutoSchedulerIntPtr(input.StatusCode)
	if strings.TrimSpace(input.Message) != "" {
		message := strings.TrimSpace(input.Message)
		state.LastError = &message
	}
	if input.CostScore != nil {
		state.CostScore = clampSignedOpenAIAutoSchedulerComponentScore(*input.CostScore)
	}
	if state.BaseScore == 0 {
		state.BaseScore = 6000
	}

	if input.EventType == OpenAIAutoSchedulerEventManualReset {
		return resetOpenAIAutoSchedulerState(now, state)
	}
	if !settings.Enabled {
		return state
	}

	state = updateOpenAIAutoSchedulerCounters(state, input)

	if state.State == OpenAIAutoSchedulerStateOpen && openAIAutoSchedulerCooldownExpired(now, state.CooldownUntil) && isOpenAIAutoSchedulerSuccessEvent(input.EventType) {
		state.State = OpenAIAutoSchedulerStateHalfOpen
		state.ConsecutiveSuccessCount = 1
		state.ConsecutiveSlowCount = 0
		state.ConsecutiveErrorCount = 0
		state.CooldownUntil = nil
		state.Reason = "cooldown expired, probing recovery"
		state.RecoveryScore = clampScore(settings.RecoveryStep)
		state.FinalScore = calculateOpenAIAutoSchedulerFinalScore(state, settings)
		return state
	}

	switch input.EventType {
	case OpenAIAutoSchedulerEventSuccess, OpenAIAutoSchedulerEventProbeSuccess:
		state.ConsecutiveSlowCount = 0
		state.ConsecutiveErrorCount = 0
		state.ConsecutiveSuccessCount++
		state.LatencyScore = latencyScoreForOpenAIAutoScheduler(input.LatencyMS, settings)
		state.ErrorScore = 0
		state.RecoveryScore = clampScore(state.RecoveryScore + settings.RecoveryStep)
		if state.State == OpenAIAutoSchedulerStateHalfOpen && state.ConsecutiveSuccessCount >= settings.HalfOpenSuccessThreshold {
			state.State = OpenAIAutoSchedulerStateRunning
			state.CooldownUntil = nil
			state.Reason = "half-open probe succeeded"
		} else if state.State != OpenAIAutoSchedulerStateHalfOpen && state.State != OpenAIAutoSchedulerStateOpen {
			state.State = OpenAIAutoSchedulerStateRunning
			state.Reason = "success"
		}
	case OpenAIAutoSchedulerEventSlow, OpenAIAutoSchedulerEventSevereSlow:
		state.ConsecutiveSlowCount++
		state.ConsecutiveErrorCount = 0
		state.ConsecutiveSuccessCount = 0
		state.LatencyScore = slowPenaltyForOpenAIAutoScheduler(input.EventType, input.LatencyMS, settings)
		state.RecoveryScore = 0
		state.Reason = input.EventType
		if state.ConsecutiveSlowCount >= settings.ConsecutiveSlowBreakerThreshold {
			state = openOpenAIAutoSchedulerCircuit(now, state, settings, "consecutive slow responses")
		} else {
			state.State = OpenAIAutoSchedulerStateObserving
		}
	case OpenAIAutoSchedulerEventError, OpenAIAutoSchedulerEventProbeError, OpenAIAutoSchedulerEventRateLimited:
		state.ConsecutiveErrorCount++
		state.ConsecutiveSlowCount = 0
		state.ConsecutiveSuccessCount = 0
		state.ErrorScore = errorPenaltyForOpenAIAutoScheduler(input.EventType)
		state.RecoveryScore = 0
		state.Reason = input.EventType
		if state.ConsecutiveErrorCount >= settings.ConsecutiveErrorBreakerThreshold || state.State == OpenAIAutoSchedulerStateHalfOpen {
			state = openOpenAIAutoSchedulerCircuit(now, state, settings, "consecutive upstream errors")
		} else {
			state.State = OpenAIAutoSchedulerStateObserving
		}
	}

	state.FinalScore = calculateOpenAIAutoSchedulerFinalScore(state, settings)
	return state
}

func updateOpenAIAutoSchedulerCounters(state OpenAIAutoSchedulerScoreState, input OpenAIAutoSchedulerEventInput) OpenAIAutoSchedulerScoreState {
	if !isOpenAIAutoSchedulerTrackedRequestEvent(input.EventType) {
		return state
	}
	previousCount := state.RequestCount
	state.RequestCount++
	if input.TtfbMS != nil && *input.TtfbMS > 0 {
		state.TtfbSampleCount++
	}
	state.SlowRate = updateOpenAIAutoSchedulerRate(state.SlowRate, previousCount, isOpenAIAutoSchedulerSlowEvent(input.EventType))
	state.ErrorRate = updateOpenAIAutoSchedulerRate(state.ErrorRate, previousCount, isOpenAIAutoSchedulerErrorEvent(input.EventType))
	state.StuckRate = updateOpenAIAutoSchedulerRate(state.StuckRate, previousCount, input.EventType == OpenAIAutoSchedulerEventSevereSlow)
	return state
}

func updateOpenAIAutoSchedulerRate(previousRate float64, previousCount int64, hit bool) float64 {
	value := 0.0
	if hit {
		value = 1
	}
	if previousCount <= 0 {
		return value
	}
	return (previousRate*float64(previousCount) + value) / float64(previousCount+1)
}

func isOpenAIAutoSchedulerTrackedRequestEvent(eventType string) bool {
	switch eventType {
	case OpenAIAutoSchedulerEventSuccess,
		OpenAIAutoSchedulerEventSlow,
		OpenAIAutoSchedulerEventSevereSlow,
		OpenAIAutoSchedulerEventError,
		OpenAIAutoSchedulerEventRateLimited,
		OpenAIAutoSchedulerEventProbeSuccess,
		OpenAIAutoSchedulerEventProbeError:
		return true
	default:
		return false
	}
}

func isOpenAIAutoSchedulerSlowEvent(eventType string) bool {
	return eventType == OpenAIAutoSchedulerEventSlow || eventType == OpenAIAutoSchedulerEventSevereSlow
}

func isOpenAIAutoSchedulerErrorEvent(eventType string) bool {
	return eventType == OpenAIAutoSchedulerEventError ||
		eventType == OpenAIAutoSchedulerEventRateLimited ||
		eventType == OpenAIAutoSchedulerEventProbeError
}

func normalizeOpenAIAutoSchedulerState(state string) string {
	switch state {
	case OpenAIAutoSchedulerStateRunning, OpenAIAutoSchedulerStateObserving, OpenAIAutoSchedulerStateOpen, OpenAIAutoSchedulerStateHalfOpen:
		return state
	default:
		return OpenAIAutoSchedulerStateRunning
	}
}

func resetOpenAIAutoSchedulerState(now time.Time, state OpenAIAutoSchedulerScoreState) OpenAIAutoSchedulerScoreState {
	state.State = OpenAIAutoSchedulerStateRunning
	state.ConsecutiveSlowCount = 0
	state.ConsecutiveErrorCount = 0
	state.ConsecutiveSuccessCount = 0
	state.CooldownUntil = nil
	state.LatencyScore = 0
	state.ErrorScore = 0
	state.RecoveryScore = 0
	state.LastCheckedAt = openAIAutoSchedulerTimePtr(now)
	state.Reason = "manual reset"
	state.FinalScore = clampScore(state.BaseScore)
	return state
}

func openOpenAIAutoSchedulerCircuit(now time.Time, state OpenAIAutoSchedulerScoreState, settings OpenAIAutoSchedulerSettings, reason string) OpenAIAutoSchedulerScoreState {
	cooldownUntil := now.Add(time.Duration(settings.CooldownSeconds) * time.Second)
	state.State = OpenAIAutoSchedulerStateOpen
	state.CooldownUntil = &cooldownUntil
	state.Reason = reason
	state.FinalScore = calculateOpenAIAutoSchedulerFinalScore(state, settings)
	return state
}

func openAIAutoSchedulerCooldownExpired(now time.Time, cooldownUntil *time.Time) bool {
	return cooldownUntil != nil && !now.Before(*cooldownUntil)
}

func isOpenAIAutoSchedulerSuccessEvent(eventType string) bool {
	return eventType == OpenAIAutoSchedulerEventSuccess || eventType == OpenAIAutoSchedulerEventProbeSuccess
}

func latencyScoreForOpenAIAutoScheduler(latencyMS *int, settings OpenAIAutoSchedulerSettings) int {
	if latencyMS == nil || *latencyMS <= 0 {
		return 0
	}
	if *latencyMS >= settings.SevereSlowThresholdMS {
		return -2500
	}
	if *latencyMS >= settings.SlowThresholdMS {
		return -1200
	}
	return 500
}

func slowPenaltyForOpenAIAutoScheduler(eventType string, latencyMS *int, settings OpenAIAutoSchedulerSettings) int {
	if eventType == OpenAIAutoSchedulerEventSevereSlow {
		return -3500
	}
	if latencyMS != nil && *latencyMS >= settings.SevereSlowThresholdMS {
		return -3500
	}
	return -2200
}

func errorPenaltyForOpenAIAutoScheduler(eventType string) int {
	if eventType == OpenAIAutoSchedulerEventRateLimited {
		return -3500
	}
	return -6000
}

func calculateOpenAIAutoSchedulerFinalScore(state OpenAIAutoSchedulerScoreState, settings OpenAIAutoSchedulerSettings) int {
	if state.State == OpenAIAutoSchedulerStateOpen {
		return clampScore(500)
	}
	costAdjustment := int(float64(state.CostScore) * settings.CostWeight)
	return clampScore(state.BaseScore + state.LatencyScore + state.ErrorScore + state.RecoveryScore + costAdjustment)
}

func clampSignedOpenAIAutoSchedulerComponentScore(score int) int {
	if score < -10000 {
		return -10000
	}
	if score > 10000 {
		return 10000
	}
	return score
}

func copyOpenAIAutoSchedulerIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func openAIAutoSchedulerTimePtr(v time.Time) *time.Time {
	return &v
}
