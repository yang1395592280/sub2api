package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type businessAnalyticsAggregationRepository struct {
	sql sqlExecutor
}

// NewBusinessAnalyticsAggregationRepository 创建经营分析聚合仓储。
func NewBusinessAnalyticsAggregationRepository(sqlDB *sql.DB) service.BusinessAnalyticsAggregationRepository {
	if sqlDB == nil {
		return nil
	}
	return newBusinessAnalyticsAggregationRepositoryWithSQL(sqlDB)
}

func newBusinessAnalyticsAggregationRepositoryWithSQL(sqlq sqlExecutor) *businessAnalyticsAggregationRepository {
	return &businessAnalyticsAggregationRepository{sql: sqlq}
}

func (r *businessAnalyticsAggregationRepository) RecomputeDaily(ctx context.Context, startDate, endDate time.Time) error {
	if r == nil || r.sql == nil || !endDate.After(startDate) {
		return nil
	}
	if db, ok := r.sql.(*sql.DB); ok {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		txRepo := newBusinessAnalyticsAggregationRepositoryWithSQL(tx)
		if err := txRepo.recomputeDailyInTx(ctx, startDate, endDate); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return r.recomputeDailyInTx(ctx, startDate, endDate)
}

func (r *businessAnalyticsAggregationRepository) RecomputeWeekly(ctx context.Context, weekStart time.Time) error {
	if r == nil || r.sql == nil || weekStart.IsZero() {
		return nil
	}
	weekEnd := weekStart.AddDate(0, 0, 7)
	if db, ok := r.sql.(*sql.DB); ok {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		txRepo := newBusinessAnalyticsAggregationRepositoryWithSQL(tx)
		if err := txRepo.recomputeWeeklyInTx(ctx, weekStart, weekEnd); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return r.recomputeWeeklyInTx(ctx, weekStart, weekEnd)
}

func (r *businessAnalyticsAggregationRepository) recomputeDailyInTx(ctx context.Context, startDate, endDate time.Time) error {
	if _, err := r.sql.ExecContext(ctx, "DELETE FROM business_usage_daily WHERE bucket_date >= $1::date AND bucket_date < $2::date", startDate, endDate); err != nil {
		return err
	}
	if _, err := r.sql.ExecContext(ctx, "DELETE FROM business_usage_daily_users WHERE bucket_date >= $1::date AND bucket_date < $2::date", startDate, endDate); err != nil {
		return err
	}
	if _, err := r.sql.ExecContext(ctx, insertBusinessUsageDailyUsersSQL, startDate, endDate); err != nil {
		return err
	}
	if _, err := r.sql.ExecContext(ctx, insertBusinessUsageDailySQL, startDate, endDate); err != nil {
		return err
	}
	return nil
}

func (r *businessAnalyticsAggregationRepository) recomputeWeeklyInTx(ctx context.Context, weekStart, weekEnd time.Time) error {
	if _, err := r.sql.ExecContext(ctx, "DELETE FROM business_usage_weekly WHERE week_start = $1::date", weekStart); err != nil {
		return err
	}
	_, err := r.sql.ExecContext(ctx, insertBusinessUsageWeeklySQL, weekStart, weekEnd)
	return err
}

const insertBusinessUsageDailyUsersSQL = `
	INSERT INTO business_usage_daily_users (
		bucket_date,
		group_id,
		account_id,
		user_id,
		requests,
		revenue,
		channel_cost,
		gross_profit
	)
	SELECT
		ul.created_at::date AS bucket_date,
		COALESCE(ul.group_id, 0) AS group_id,
		COALESCE(ul.account_id, 0) AS account_id,
		ul.user_id,
		COUNT(*) AS requests,
		COALESCE(SUM(ul.actual_cost), 0) AS revenue,
		` + businessUsageChannelCostSumExpr + ` AS channel_cost,
		` + businessUsageChannelGrossProfitSumExpr + ` AS gross_profit
	FROM usage_logs ul
	WHERE ul.created_at >= $1 AND ul.created_at < $2
	GROUP BY 1, 2, 3, 4
`

const insertBusinessUsageDailySQL = `
	INSERT INTO business_usage_daily (
		bucket_date,
		group_id,
		account_id,
		channel_id,
		platform,
		requests,
		active_users,
		active_api_keys,
		total_tokens,
		revenue,
		channel_cost,
		gross_profit,
		avg_group_rate_multiplier,
		avg_channel_price,
		missing_channel_price_records,
		computed_at
	)
	SELECT
		ul.created_at::date AS bucket_date,
		COALESCE(ul.group_id, 0) AS group_id,
		COALESCE(ul.account_id, 0) AS account_id,
		COALESCE(MIN(ul.channel_id), 0) AS channel_id,
		COALESCE(MIN(` + usageLogEffectivePlatformExpr + `) FILTER (WHERE ` + usageLogEffectivePlatformExpr + ` IS NOT NULL), '') AS platform,
		COUNT(*) AS requests,
		COUNT(DISTINCT ul.user_id) AS active_users,
		COUNT(DISTINCT ul.api_key_id) AS active_api_keys,
		COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens), 0) AS total_tokens,
		COALESCE(SUM(ul.actual_cost), 0) AS revenue,
		` + businessUsageChannelCostSumExpr + ` AS channel_cost,
		` + businessUsageChannelGrossProfitSumExpr + ` AS gross_profit,
		CASE WHEN COUNT(*) > 0 THEN SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) / SUM(GREATEST(ul.actual_cost, 0.000000001)) END AS avg_group_rate_multiplier,
		AVG(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL) AS avg_channel_price,
		COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL) AS missing_channel_price_records,
		NOW() AS computed_at
	FROM usage_logs ul
	LEFT JOIN groups g ON g.id = ul.group_id
	LEFT JOIN accounts a ON a.id = ul.account_id
	WHERE ul.created_at >= $1 AND ul.created_at < $2
	GROUP BY 1, 2, 3
`

const insertBusinessUsageWeeklySQL = `
	INSERT INTO business_usage_weekly (
		week_start,
		group_id,
		account_id,
		channel_id,
		platform,
		requests,
		active_users,
		active_api_keys,
		total_tokens,
		revenue,
		channel_cost,
		gross_profit,
		avg_group_rate_multiplier,
		avg_channel_price,
		missing_channel_price_records,
		computed_at
	)
	SELECT
		$1::date AS week_start,
		COALESCE(ul.group_id, 0) AS group_id,
		COALESCE(ul.account_id, 0) AS account_id,
		COALESCE(MIN(ul.channel_id), 0) AS channel_id,
		COALESCE(MIN(` + usageLogEffectivePlatformExpr + `) FILTER (WHERE ` + usageLogEffectivePlatformExpr + ` IS NOT NULL), '') AS platform,
		COUNT(*) AS requests,
		COUNT(DISTINCT ul.user_id) AS active_users,
		COUNT(DISTINCT ul.api_key_id) AS active_api_keys,
		COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens), 0) AS total_tokens,
		COALESCE(SUM(ul.actual_cost), 0) AS revenue,
		` + businessUsageChannelCostSumExpr + ` AS channel_cost,
		` + businessUsageChannelGrossProfitSumExpr + ` AS gross_profit,
		CASE WHEN COUNT(*) > 0 THEN SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) / SUM(GREATEST(ul.actual_cost, 0.000000001)) END AS avg_group_rate_multiplier,
		AVG(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL) AS avg_channel_price,
		COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL) AS missing_channel_price_records,
		NOW() AS computed_at
	FROM usage_logs ul
	LEFT JOIN groups g ON g.id = ul.group_id
	LEFT JOIN accounts a ON a.id = ul.account_id
	WHERE ul.created_at >= $1 AND ul.created_at < $2
	GROUP BY 2, 3
`
