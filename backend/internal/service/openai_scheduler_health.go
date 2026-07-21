package service

import (
	"context"
	"time"
)

const (
	OpenAISchedulerHealthConfidenceHigh   = "high"
	OpenAISchedulerHealthConfidenceMedium = "medium"
	OpenAISchedulerHealthConfidenceLow    = "low"
)

type OpenAISchedulerHealthKey struct {
	AccountID   int64
	ModelFamily string
	Endpoint    string
	Transport   string
}

type OpenAISchedulerHealthSnapshot struct {
	Key                OpenAISchedulerHealthKey
	State              string
	PredictedTTFTMS    float64
	ErrorRate          float64
	RateLimitedRate    float64
	ServerErrorRate    float64
	ConsecutiveSlow    int
	ConsecutiveError   int
	ConsecutiveSuccess int
	RealSampleCount    int64
	ProbeSampleCount   int64
	LastRealAt         *time.Time
	LastProbeAt        *time.Time
	CooldownUntil      *time.Time
	ExpiresAt          time.Time
}

type OpenAISchedulerHealthRepository interface {
	GetBatch(context.Context, []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error)
	Upsert(context.Context, OpenAISchedulerHealthSnapshot) error
}

// OpenAISchedulerHealthSummaryRepository provides the bounded account-level
// read used by the account management page. Scheduling hot paths still use
// exact health keys through OpenAISchedulerHealthRepository.GetBatch.
type OpenAISchedulerHealthSummaryRepository interface {
	ListByAccountIDs(context.Context, []int64) ([]OpenAISchedulerHealthSnapshot, error)
}

func classifyOpenAISchedulerHealthConfidence(
	state string,
	expiresAt time.Time,
	realSampleCount int64,
	probeSampleCount int64,
	lastRealAt *time.Time,
	now time.Time,
	realSampleFreshSeconds int,
) string {
	if now.IsZero() {
		now = time.Now()
	}
	if expiresAt.IsZero() || !now.Before(expiresAt) {
		return OpenAISchedulerHealthConfidenceLow
	}

	switch normalizeOpenAIAutoSchedulerState(state) {
	case OpenAIAutoSchedulerStateOpen, OpenAIAutoSchedulerStateHalfOpen, OpenAIAutoSchedulerStateObserving:
		return OpenAISchedulerHealthConfidenceLow
	}

	if realSampleFreshSeconds <= 0 {
		realSampleFreshSeconds = DefaultOpenAIAutoSchedulerSettings().RealSampleFreshSeconds
	}
	if realSampleCount > 0 && lastRealAt != nil {
		age := now.Sub(*lastRealAt)
		if age < 0 {
			age = 0
		}
		if age <= time.Duration(realSampleFreshSeconds)*time.Second {
			return OpenAISchedulerHealthConfidenceHigh
		}
	}
	if realSampleCount+probeSampleCount > 0 {
		return OpenAISchedulerHealthConfidenceMedium
	}
	return OpenAISchedulerHealthConfidenceLow
}
