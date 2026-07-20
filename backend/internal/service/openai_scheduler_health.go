package service

import (
	"context"
	"time"
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
