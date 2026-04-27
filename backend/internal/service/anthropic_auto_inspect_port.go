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
	AnthropicAutoInspectBatchStatusSkipped   = "skipped"
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

type CreateAnthropicAutoInspectSkippedBatchInput struct {
	TriggerSource string
	SkipReason    string
	StartedAt     time.Time
	FinishedAt    time.Time
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
	ID                int64      `json:"id"`
	TriggerSource     string     `json:"trigger_source"`
	Status            string     `json:"status"`
	SkipReason        string     `json:"skip_reason"`
	TotalAccounts     int        `json:"total_accounts"`
	ProcessedAccounts int        `json:"processed_accounts"`
	SuccessCount      int        `json:"success_count"`
	RateLimitedCount  int        `json:"rate_limited_count"`
	ErrorCount        int        `json:"error_count"`
	SkippedCount      int        `json:"skipped_count"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

type AnthropicAutoInspectLog struct {
	ID                     int64                      `json:"id"`
	BatchID                int64                      `json:"batch_id"`
	AccountID              int64                      `json:"account_id"`
	AccountNameSnapshot    string                     `json:"account_name_snapshot"`
	Platform               string                     `json:"platform"`
	AccountType            string                     `json:"account_type"`
	Result                 AnthropicAutoInspectResult `json:"result"`
	SkipReason             string                     `json:"skip_reason"`
	ResponseText           string                     `json:"response_text"`
	ErrorMessage           string                     `json:"error_message"`
	RateLimitResetAt       *time.Time                 `json:"rate_limit_reset_at"`
	TempUnschedulableUntil *time.Time                 `json:"temp_unschedulable_until"`
	SchedulableChanged     bool                       `json:"schedulable_changed"`
	StartedAt              time.Time                  `json:"started_at"`
	FinishedAt             time.Time                  `json:"finished_at"`
	LatencyMs              int64                      `json:"latency_ms"`
	CreatedAt              time.Time                  `json:"created_at"`
}

type AnthropicAutoInspectLogFilter struct {
	Result      AnthropicAutoInspectResult
	Search      string
	StartedFrom *time.Time
	StartedTo   *time.Time
}

type AnthropicAutoInspectSettings struct {
	Enabled              bool `json:"enabled"`
	IntervalMinutes      int  `json:"interval_minutes"`
	ErrorCooldownMinutes int  `json:"error_cooldown_minutes"`
}

type AnthropicAutoInspectRepository interface {
	CreateBatch(ctx context.Context, input CreateAnthropicAutoInspectBatchInput) (int64, error)
	CreateSkippedBatch(ctx context.Context, input CreateAnthropicAutoInspectSkippedBatchInput) (int64, error)
	CompleteBatch(ctx context.Context, batchID int64, stats AnthropicAutoInspectBatchStats, finishedAt time.Time) error
	MarkBatchFailed(ctx context.Context, batchID int64, stats AnthropicAutoInspectBatchStats, finishedAt time.Time) error
	CreateLog(ctx context.Context, log AnthropicAutoInspectLog) error
	ListLogs(ctx context.Context, params pagination.PaginationParams, filter AnthropicAutoInspectLogFilter) ([]AnthropicAutoInspectLog, *pagination.PaginationResult, error)
	ListBatches(ctx context.Context, params pagination.PaginationParams) ([]AnthropicAutoInspectBatch, *pagination.PaginationResult, error)
}
