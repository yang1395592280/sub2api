package service

import (
	"context"
	"time"
)

// BusinessAnalyticsAggregationRepository 负责重建经营分析聚合表。
type BusinessAnalyticsAggregationRepository interface {
	RecomputeDaily(ctx context.Context, startDate, endDate time.Time) error
	RecomputeWeekly(ctx context.Context, weekStart time.Time) error
}
