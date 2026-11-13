package service

import (
	"context"
	"time"
)

type PelicanTest struct {
	TestKind               string     `json:"test_kind"`
	Answer                 string     `json:"answer"`
	EstimatedCostUSD       *float64   `json:"estimated_cost_usd"`
	UpstreamRateMultiplier *float64   `json:"upstream_rate_multiplier"`
	CostModelID            string     `json:"cost_model_id"`
	ID                     int64      `json:"id"`
	BatchID                string     `json:"batch_id"`
	AccountID              int64      `json:"account_id"`
	ModelID                string     `json:"model_id"`
	Prompt                 string     `json:"prompt"`
	Status                 string     `json:"status"`
	ResponseText           string     `json:"response_text"`
	HTML                   string     `json:"html"`
	ErrorMessage           string     `json:"error_message"`
	LatencyMS              int64      `json:"latency_ms"`
	InputTokens            *int64     `json:"input_tokens"`
	OutputTokens           *int64     `json:"output_tokens"`
	TotalTokens            *int64     `json:"total_tokens"`
	CreatedAt              time.Time  `json:"created_at"`
	StartedAt              *time.Time `json:"started_at,omitempty"`
	FinishedAt             *time.Time `json:"finished_at,omitempty"`
	SourceGroupID          int64      `json:"source_group_id"`
	SourceGroupName        string     `json:"source_group_name"`
}

const DefaultPelicanTestPrompt = "创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试"

const (
	TestKindPelican               = "pelican"
	TestKindIntelligence          = "intelligence"
	DefaultIntelligenceTestPrompt = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。已知不同口味的糖果和不同形状的数量统计如下表。最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（手中有圆形苹果味配五角星桃子味，或有圆形桃子味配五角星苹果味都满足）

| | 苹果味 | 桃子味 | 西瓜味 |
|---|---|---|---|
| 圆形 | 7 | 9 | 8 |
| 五角星形 | 6 | 6 | 4 |

请给出最终答案数字。
不要联网。`
)

// GenerationTestKind keeps existing callers and records on the pelican path.
func GenerationTestKind(kinds ...string) string {
	if len(kinds) > 0 && kinds[0] != "" {
		return kinds[0]
	}
	return TestKindPelican
}

func GenerationTestPrompt(kind string) string {
	if kind == TestKindIntelligence {
		return DefaultIntelligenceTestPrompt
	}
	return DefaultPelicanTestPrompt
}

type GroupPelicanConfig struct {
	TestKind        string     `json:"test_kind"`
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
	CreateBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, prompt string, kinds ...string) ([]PelicanTest, error)
	MarkRunning(ctx context.Context, id int64) (bool, error)
	Finish(ctx context.Context, id int64, status, responseText, html, errorMessage string, latencyMS int64, usage AccountTestTokenUsage, details ...PelicanTest) error
	ListLatest(ctx context.Context, accountIDs []int64, kinds ...string) ([]PelicanTest, error)
	Get(ctx context.Context, id int64, kinds ...string) (*PelicanTest, error)
	ListHistory(ctx context.Context, accountID int64, page, pageSize int, kinds ...string) (*PelicanTestHistory, error)
	GetGroupConfig(ctx context.Context, groupID int64, kinds ...string) (*GroupPelicanConfig, error)
	SaveGroupConfig(ctx context.Context, config GroupPelicanConfig) error
	EnqueueDueGroups(ctx context.Context) error
	ClaimNext(ctx context.Context) (*PelicanTest, error)
	ScheduledTestEnabled(ctx context.Context, test PelicanTest) (bool, error)
}
