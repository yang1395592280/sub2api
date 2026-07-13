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
