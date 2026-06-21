package service

import (
	"context"
	"time"
)

type AccountStabilityStats struct {
	SuccessCount  int64
	ErrorCount    int64
	AvgDurationMs *int
}

type AccountStability struct {
	Level         string   `json:"level"`
	Label         string   `json:"label"`
	SuccessRate   *float64 `json:"success_rate,omitempty"`
	TotalRequests int64    `json:"total_requests"`
	SuccessCount  int64    `json:"success_count"`
	ErrorCount    int64    `json:"error_count"`
	AvgDurationMs *int     `json:"avg_duration_ms,omitempty"`
	WindowDays    int      `json:"window_days"`
	Reason        string   `json:"reason,omitempty"`
}

type accountStabilityStatsBatchReader interface {
	GetAccountStabilityStatsBatch(ctx context.Context, accountIDs []int64, startTime, endTime time.Time) (map[int64]*AccountStabilityStats, error)
}

func BuildAccountStability(account *Account, stats *AccountStabilityStats, windowDays int) *AccountStability {
	if stats == nil {
		stats = &AccountStabilityStats{}
	}
	if windowDays <= 0 {
		windowDays = 3
	}

	total := stats.SuccessCount + stats.ErrorCount
	stability := &AccountStability{
		Level:         "unknown",
		Label:         "无数据",
		TotalRequests: total,
		SuccessCount:  stats.SuccessCount,
		ErrorCount:    stats.ErrorCount,
		AvgDurationMs: stats.AvgDurationMs,
		WindowDays:    windowDays,
	}
	if account == nil || !account.IsSchedulable() {
		stability.Level = "down"
		stability.Label = "不通"
		stability.Reason = "账号当前不可调度"
		return stability
	}
	if total <= 0 {
		return stability
	}

	successRate := float64(stats.SuccessCount) / float64(total)
	stability.SuccessRate = &successRate
	switch {
	case successRate >= 0.99 && (stats.AvgDurationMs == nil || *stats.AvgDurationMs <= 2500):
		stability.Level = "excellent"
		stability.Label = "优秀"
	case successRate >= 0.95:
		stability.Level = "healthy"
		stability.Label = "健康"
	case successRate >= 0.80:
		stability.Level = "normal"
		stability.Label = "一般"
	default:
		stability.Level = "down"
		stability.Label = "不通"
	}
	return stability
}
