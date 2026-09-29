package service

import (
	"context"
	"time"
)

type PelicanTest struct {
	ID           int64      `json:"id"`
	BatchID      string     `json:"batch_id"`
	AccountID    int64      `json:"account_id"`
	ModelID      string     `json:"model_id"`
	Prompt       string     `json:"prompt"`
	Status       string     `json:"status"`
	ResponseText string     `json:"response_text"`
	HTML         string     `json:"html"`
	ErrorMessage string     `json:"error_message"`
	LatencyMS    int64      `json:"latency_ms"`
	InputTokens  *int64     `json:"input_tokens"`
	OutputTokens *int64     `json:"output_tokens"`
	TotalTokens  *int64     `json:"total_tokens"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type PelicanTestRepository interface {
	ExpireStale(ctx context.Context) error
	CreateBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, prompt string) ([]PelicanTest, error)
	MarkRunning(ctx context.Context, id int64) (bool, error)
	Finish(ctx context.Context, id int64, status, responseText, html, errorMessage string, latencyMS int64, usage AccountTestTokenUsage) error
	ListLatest(ctx context.Context, accountIDs []int64) ([]PelicanTest, error)
	Get(ctx context.Context, id int64) (*PelicanTest, error)
}
