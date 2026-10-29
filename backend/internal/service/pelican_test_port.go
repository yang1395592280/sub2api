package service

import (
	"context"
	"time"
)

type PelicanTest struct {
	ID              int64      `json:"id"`
	BatchID         string     `json:"batch_id"`
	AccountID       int64      `json:"account_id"`
	ModelID         string     `json:"model_id"`
	Prompt          string     `json:"prompt"`
	Status          string     `json:"status"`
	ResponseText    string     `json:"response_text"`
	HTML            string     `json:"html"`
	ErrorMessage    string     `json:"error_message"`
	LatencyMS       int64      `json:"latency_ms"`
	InputTokens     *int64     `json:"input_tokens"`
	OutputTokens    *int64     `json:"output_tokens"`
	TotalTokens     *int64     `json:"total_tokens"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	SourceGroupID   int64      `json:"source_group_id"`
	SourceGroupName string     `json:"source_group_name"`
}

const DefaultPelicanTestPrompt = "创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试"

type GroupPelicanConfig struct {
	GroupID         int64      `json:"group_id"`
	Enabled         bool       `json:"enabled"`
	ModelID         string     `json:"model_id"`
	Prompt          string     `json:"prompt"`
	IntervalMinutes int        `json:"interval_minutes"`
	LastRunAt       *time.Time `json:"last_run_at"`
	NextRunAt       *time.Time `json:"next_run_at"`
}

type PelicanTestHistory struct {
	Items    []PelicanTest `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type PelicanTestRepository interface {
	ExpireStale(ctx context.Context) error
	CreateBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, prompt string) ([]PelicanTest, error)
	MarkRunning(ctx context.Context, id int64) (bool, error)
	Finish(ctx context.Context, id int64, status, responseText, html, errorMessage string, latencyMS int64, usage AccountTestTokenUsage) error
	ListLatest(ctx context.Context, accountIDs []int64) ([]PelicanTest, error)
	Get(ctx context.Context, id int64) (*PelicanTest, error)
	ListHistory(ctx context.Context, accountID int64, page, pageSize int) (*PelicanTestHistory, error)
	GetGroupConfig(ctx context.Context, groupID int64) (*GroupPelicanConfig, error)
	SaveGroupConfig(ctx context.Context, config GroupPelicanConfig) error
	EnqueueDueGroups(ctx context.Context) error
	ClaimNext(ctx context.Context) (*PelicanTest, error)
	ScheduledTestEnabled(ctx context.Context, test PelicanTest) (bool, error)
}
