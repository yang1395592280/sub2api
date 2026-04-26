package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	AnthropicAutoInspectTriggerSourceScheduler = "scheduler"
	AnthropicAutoInspectTriggerSourceManual    = "manual"
)

const (
	AnthropicAutoInspectBatchStatusRunning   = "running"
	AnthropicAutoInspectBatchStatusCompleted = "completed"
	AnthropicAutoInspectBatchStatusFailed    = "failed"
)

type AnthropicAutoInspectResult string

const (
	AnthropicAutoInspectResultSuccess     AnthropicAutoInspectResult = "success"
	AnthropicAutoInspectResultRateLimited AnthropicAutoInspectResult = "rate_limited"
	AnthropicAutoInspectResultError       AnthropicAutoInspectResult = "error"
	AnthropicAutoInspectResultSkipped     AnthropicAutoInspectResult = "skipped"
)

type CreateAnthropicAutoInspectBatchInput struct {
	TriggerSource string
	Status        string
	StartedAt     time.Time
}

type AnthropicAutoInspectBatchStats struct {
	TotalAccounts     int
	ProcessedAccounts int
	SuccessCount      int
	RateLimitedCount  int
	ErrorCount        int
	SkippedCount      int
}

type AnthropicAutoInspectBatch struct {
	ID                int64
	TriggerSource     string
	Status            string
	TotalAccounts     int
	ProcessedAccounts int
	SuccessCount      int
	RateLimitedCount  int
	ErrorCount        int
	SkippedCount      int
	StartedAt         time.Time
	FinishedAt        *time.Time
	CreatedAt         time.Time
}

type AnthropicAutoInspectLog struct {
	ID                     int64
	BatchID                int64
	AccountID              int64
	AccountNameSnapshot    string
	Platform               string
	AccountType            string
	Result                 AnthropicAutoInspectResult
	SkipReason             string
	ResponseText           string
	ErrorMessage           string
	RateLimitResetAt       *time.Time
	TempUnschedulableUntil *time.Time
	SchedulableChanged     bool
	StartedAt              time.Time
	FinishedAt             time.Time
	LatencyMs              int64
	CreatedAt              time.Time
}

type AnthropicAutoInspectLogFilter struct {
	Result AnthropicAutoInspectResult
	Search string
}

type AnthropicAutoInspectSettings struct {
	Enabled              bool `json:"enabled"`
	IntervalMinutes      int  `json:"interval_minutes"`
	ErrorCooldownMinutes int  `json:"error_cooldown_minutes"`
}

type AnthropicAutoInspectRepository interface {
	CreateBatch(ctx context.Context, input CreateAnthropicAutoInspectBatchInput) (int64, error)
	CompleteBatch(ctx context.Context, batchID int64, stats AnthropicAutoInspectBatchStats, finishedAt time.Time) error
	MarkBatchFailed(ctx context.Context, batchID int64, stats AnthropicAutoInspectBatchStats, finishedAt time.Time) error
	CreateLog(ctx context.Context, log AnthropicAutoInspectLog) error
	ListLogs(ctx context.Context, params pagination.PaginationParams, filter AnthropicAutoInspectLogFilter) ([]AnthropicAutoInspectLog, *pagination.PaginationResult, error)
	ListBatches(ctx context.Context, params pagination.PaginationParams) ([]AnthropicAutoInspectBatch, *pagination.PaginationResult, error)
}
