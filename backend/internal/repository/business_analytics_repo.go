package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type businessAnalyticsRepository struct {
	sql sqlExecutor
}

const (
	businessUsageBaseCostExpr              = "COALESCE(COALESCE(ul.actual_cost, 0) / NULLIF(ul.rate_multiplier, 0), COALESCE(ul.account_stats_cost, ul.total_cost, 0))"
	businessUsageChannelCostExpr           = businessUsageBaseCostExpr + " * COALESCE(ul.account_rate_multiplier, 1)"
	businessUsageChannelCostSumExpr        = "COALESCE(SUM(" + businessUsageChannelCostExpr + "), 0)"
	businessUsageChannelGrossProfitSumExpr = "COALESCE(SUM(ul.actual_cost), 0) - " + businessUsageChannelCostSumExpr
	businessUsageRecordChannelCostExpr     = "COALESCE(" + businessUsageChannelCostExpr + ", 0)"
	businessUsageRecordGrossProfitExpr     = "COALESCE(ul.actual_cost, 0) - " + businessUsageRecordChannelCostExpr
)

func NewBusinessAnalyticsRepository(sqlDB *sql.DB) service.BusinessAnalyticsRepository {
	if sqlDB == nil {
		return nil
	}
	return newBusinessAnalyticsRepositoryWithSQL(sqlDB)
}

func newBusinessAnalyticsRepositoryWithSQL(sqlq sqlExecutor) *businessAnalyticsRepository {
	return &businessAnalyticsRepository{sql: sqlq}
}

func (r *businessAnalyticsRepository) GetOverview(ctx context.Context, filter service.BusinessAnalyticsFilter) (*service.BusinessOverviewData, error) {
	query, args := buildBusinessAggregateQuery(filter, "", false)
	var data service.BusinessOverviewData
	if err := scanSingleRow(ctx, r.sql, query, args, &data.Requests, &data.ActiveUsers, &data.ActiveAPIKeys, &data.TotalTokens, &data.Revenue, &data.ChannelCost, &data.GrossProfit, &data.MissingPrice); err != nil {
		return nil, err
	}
	return &data, nil
}

func (r *businessAnalyticsRepository) GetTrend(ctx context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessTrendPoint, error) {
	query, args := buildBusinessAggregateQuery(filter, "bucket_date", true)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []service.BusinessTrendPoint
	for rows.Next() {
		var point service.BusinessTrendPoint
		if err := rows.Scan(&point.Date, &point.Requests, &point.ActiveUsers, &point.Revenue, &point.ChannelCost, &point.GrossProfit); err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (r *businessAnalyticsRepository) GetGroups(ctx context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessGroupRow, error) {
	periodDays := int(filter.EndDate.Sub(filter.StartDate).Hours() / 24)
	if periodDays <= 0 {
		periodDays = 1
	}
	previous := filter
	previous.EndDate = filter.StartDate
	previous.StartDate = filter.StartDate.AddDate(0, 0, -periodDays)

	query, args := buildBusinessGroupsQuery(filter, previous)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []service.BusinessGroupRow
	for rows.Next() {
		var row service.BusinessGroupRow
		if err := rows.Scan(
			&row.GroupID,
			&row.GroupName,
			&row.Platform,
			&row.CurrentRateMultiplier,
			&row.AverageRateMultiplier,
			&row.Requests,
			&row.ActiveUsers,
			&row.ActiveAPIKeys,
			&row.TotalTokens,
			&row.Revenue,
			&row.ChannelCost,
			&row.GrossProfit,
			&row.PreviousRevenue,
			&row.PreviousGrossProfit,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *businessAnalyticsRepository) GetChannels(ctx context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessChannelRow, error) {
	query, args := buildBusinessChannelsQuery(filter)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []service.BusinessChannelRow
	for rows.Next() {
		var row service.BusinessChannelRow
		if err := rows.Scan(
			&row.AccountID,
			&row.AccountName,
			&row.ChannelID,
			&row.Platform,
			&row.Status,
			&row.CurrentChannelPrice,
			&row.BalanceStatus,
			&row.AverageChannelPrice,
			&row.Requests,
			&row.ActiveUsers,
			&row.ActiveAPIKeys,
			&row.TotalTokens,
			&row.Revenue,
			&row.ChannelCost,
			&row.GrossProfit,
			&row.MissingPriceRecords,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *businessAnalyticsRepository) GetPriceChangeImpact(ctx context.Context, input service.PriceChangeImpactInput) (*service.PriceChangeImpactResponse, error) {
	days := input.Days
	if days <= 0 {
		days = 7
	}
	query, args := buildPriceChangeImpactQuery(input)
	var resp service.PriceChangeImpactResponse
	resp.GroupID = input.GroupID
	resp.ChangeDate = input.ChangeDate.Format("2006-01-02")
	resp.ChangeAt = input.ChangeDate
	err := scanSingleRow(ctx, r.sql, query, args,
		&resp.BeforeRequests,
		&resp.AfterRequests,
		&resp.BeforeActiveUsers,
		&resp.AfterActiveUsers,
		&resp.BeforeRevenue,
		&resp.AfterRevenue,
		&resp.RevenueDelta,
		&resp.BeforeChannelCost,
		&resp.AfterChannelCost,
		&resp.BeforeGrossProfit,
		&resp.AfterGrossProfit,
		&resp.GrossProfitDelta,
		&resp.BeforeAvgRateMultiplier,
		&resp.AfterAvgRateMultiplier,
		&resp.NewUsers,
		&resp.LostUsers,
	)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func buildPriceChangeImpactQuery(input service.PriceChangeImpactInput) (string, []any) {
	days := input.Days
	if days <= 0 {
		days = 7
	}
	beforeStart := input.ChangeDate.AddDate(0, 0, -days)
	beforeEnd := input.ChangeDate
	afterStart := input.ChangeDate
	afterEnd := input.ChangeDate.AddDate(0, 0, days)
	query := `
WITH before_period AS (
	SELECT
		COALESCE(SUM(requests), 0) AS requests,
		COALESCE(SUM(revenue), 0) AS revenue,
		COALESCE(SUM(channel_cost), 0) AS channel_cost,
		COALESCE(SUM(gross_profit), 0) AS gross_profit,
		SUM(avg_group_rate_multiplier * GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL)
			/ NULLIF(SUM(GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL), 0) AS avg_rate_multiplier
	FROM business_usage_daily
	WHERE bucket_date >= $1::date AND bucket_date < $2::date AND group_id = $5
), after_period AS (
	SELECT
		COALESCE(SUM(requests), 0) AS requests,
		COALESCE(SUM(revenue), 0) AS revenue,
		COALESCE(SUM(channel_cost), 0) AS channel_cost,
		COALESCE(SUM(gross_profit), 0) AS gross_profit,
		SUM(avg_group_rate_multiplier * GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL)
			/ NULLIF(SUM(GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL), 0) AS avg_rate_multiplier
	FROM business_usage_daily
	WHERE bucket_date >= $3::date AND bucket_date < $4::date AND group_id = $5
), before_users AS (
	SELECT DISTINCT user_id
	FROM business_usage_daily_users
	WHERE bucket_date >= $1::date AND bucket_date < $2::date AND group_id = $5
), after_users AS (
	SELECT DISTINCT user_id
	FROM business_usage_daily_users
	WHERE bucket_date >= $3::date AND bucket_date < $4::date AND group_id = $5
)
SELECT
	before_period.requests AS before_requests,
	after_period.requests AS after_requests,
	(SELECT COUNT(*) FROM before_users) AS before_active_users,
	(SELECT COUNT(*) FROM after_users) AS after_active_users,
	before_period.revenue AS before_revenue,
	after_period.revenue AS after_revenue,
	after_period.revenue - before_period.revenue AS revenue_delta,
	before_period.channel_cost AS before_channel_cost,
	after_period.channel_cost AS after_channel_cost,
	before_period.gross_profit AS before_gross_profit,
	after_period.gross_profit AS after_gross_profit,
	after_period.gross_profit - before_period.gross_profit AS gross_profit_delta,
	before_period.avg_rate_multiplier AS before_avg_rate_multiplier,
	after_period.avg_rate_multiplier AS after_avg_rate_multiplier,
	(SELECT COUNT(*) FROM after_users au LEFT JOIN before_users bu ON bu.user_id = au.user_id WHERE bu.user_id IS NULL) AS new_users,
	(SELECT COUNT(*) FROM before_users bu LEFT JOIN after_users au ON au.user_id = bu.user_id WHERE au.user_id IS NULL) AS lost_users
FROM before_period, after_period`
	return query, []any{beforeStart, beforeEnd, afterStart, afterEnd, input.GroupID}
}

func (r *businessAnalyticsRepository) GetRecords(ctx context.Context, filter service.BusinessRecordsFilter) (*service.BusinessRecordsResponse, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	where, args := buildUsageLogsWhere(filter.BusinessAnalyticsFilter)
	countQuery := "SELECT COUNT(*) FROM usage_logs ul LEFT JOIN groups g ON g.id = ul.group_id LEFT JOIN accounts a ON a.id = ul.account_id " + where
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := `
SELECT
	ul.id,
	ul.created_at,
	ul.user_id,
	COALESCE(u.email, ''),
	ul.api_key_id,
	COALESCE(ak.name, ''),
	COALESCE(ul.group_id, 0),
	COALESCE(g.name, ''),
	COALESCE(ul.account_id, 0),
	COALESCE(a.name, ''),
	ul.model,
	1::bigint AS requests,
	COALESCE(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens, 0) AS total_tokens,
	COALESCE(ul.actual_cost, 0) AS revenue,
	` + businessUsageRecordChannelCostExpr + ` AS channel_cost,
	` + businessUsageRecordGrossProfitExpr + ` AS gross_profit,
	ul.rate_multiplier,
	ul.channel_price_snapshot,
	ul.channel_price_snapshot IS NULL AS channel_price_snapshot_missing
FROM usage_logs ul
LEFT JOIN users u ON u.id = ul.user_id
LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
` + where + `
ORDER BY ul.created_at DESC, ul.id DESC
LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &service.BusinessRecordsResponse{Total: total, Page: filter.Page, PageSize: filter.PageSize}
	for rows.Next() {
		var row service.BusinessRecordRow
		if err := rows.Scan(
			&row.ID,
			&row.CreatedAt,
			&row.UserID,
			&row.UserEmail,
			&row.APIKeyID,
			&row.APIKeyName,
			&row.GroupID,
			&row.GroupName,
			&row.AccountID,
			&row.AccountName,
			&row.Model,
			&row.Requests,
			&row.TotalTokens,
			&row.Revenue,
			&row.ChannelCost,
			&row.GrossProfit,
			&row.RateMultiplier,
			&row.ChannelPriceSnapshot,
			&row.ChannelPriceSnapshotMissing,
		); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, row)
	}
	return out, rows.Err()
}

func buildBusinessAggregateQuery(filter service.BusinessAnalyticsFilter, groupBy string, trend bool) (string, []any) {
	if includesToday(filter) {
		return buildUsageLogAggregateQuery(filter, trend)
	}
	tableName := "business_usage_daily"
	dateColumn := "bucket_date"
	_, args := buildBusinessDailyWhere(filter, dateColumn)
	dailyWhereWithAlias, _ := buildBusinessDailyWhere(filter, "b.bucket_date")
	aggregateWhereWithAlias, _ := buildBusinessDailyWhere(filter, "b."+dateColumn)
	usageWhereWithAlias, _ := buildUsageLogsWhere(filter)
	if trend {
		if isWeeklyGranularity(filter) {
			return `
WITH usage_totals AS (
	SELECT
		date_trunc('week', b.bucket_date)::date::text date,
		COALESCE(SUM(b.requests), 0) requests,
		COALESCE(SUM(b.revenue), 0) revenue,
		COALESCE(SUM(b.channel_cost), 0) channel_cost,
		COALESCE(SUM(b.gross_profit), 0) gross_profit
	FROM business_usage_daily b ` + aggregateWhereWithAlias + `
	GROUP BY date_trunc('week', b.bucket_date)::date
), active_users AS (
	SELECT
		date_trunc('week', bu.bucket_date)::date::text date,
		COUNT(DISTINCT bu.user_id) active_users
	FROM business_usage_daily_users bu
	JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id
	` + dailyWhereWithAlias + `
	GROUP BY date_trunc('week', bu.bucket_date)::date
)
SELECT usage_totals.date, usage_totals.requests, COALESCE(active_users.active_users, 0), usage_totals.revenue, usage_totals.channel_cost, usage_totals.gross_profit
FROM usage_totals
LEFT JOIN active_users ON active_users.date = usage_totals.date
ORDER BY usage_totals.date`, args
		}
		return `
WITH usage_totals AS (
	SELECT
		b.bucket_date::text date,
		COALESCE(SUM(b.requests), 0) requests,
		COALESCE(SUM(b.revenue), 0) revenue,
		COALESCE(SUM(b.channel_cost), 0) channel_cost,
		COALESCE(SUM(b.gross_profit), 0) gross_profit
	FROM business_usage_daily b ` + dailyWhereWithAlias + `
	GROUP BY b.bucket_date
), active_users AS (
	SELECT bu.bucket_date::text date, COUNT(DISTINCT bu.user_id) active_users
	FROM business_usage_daily_users bu
	JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id
	` + dailyWhereWithAlias + `
	GROUP BY bu.bucket_date
)
SELECT usage_totals.date, usage_totals.requests, COALESCE(active_users.active_users, 0), usage_totals.revenue, usage_totals.channel_cost, usage_totals.gross_profit
FROM usage_totals
LEFT JOIN active_users ON active_users.date = usage_totals.date
ORDER BY usage_totals.date`, args
	}
	selectCols := `
COALESCE(usage_totals.requests, 0),
COALESCE(active_users.active_users, 0),
COALESCE(active_api_keys.active_api_keys, 0),
COALESCE(usage_totals.total_tokens, 0),
COALESCE(usage_totals.revenue, 0),
COALESCE(usage_totals.channel_cost, 0),
COALESCE(usage_totals.gross_profit, 0),
COALESCE(usage_totals.missing_channel_price_records, 0)`
	return `
WITH usage_totals AS (
	SELECT
		COALESCE(SUM(requests), 0) requests,
		COALESCE(SUM(total_tokens), 0) total_tokens,
		COALESCE(SUM(revenue), 0) revenue,
		COALESCE(SUM(channel_cost), 0) channel_cost,
		COALESCE(SUM(gross_profit), 0) gross_profit,
		COALESCE(SUM(missing_channel_price_records), 0) missing_channel_price_records
	FROM ` + tableName + ` b ` + aggregateWhereWithAlias + `
), active_users AS (
	SELECT COUNT(DISTINCT bu.user_id) active_users
	FROM business_usage_daily_users bu
	JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id
	` + dailyWhereWithAlias + `
), active_api_keys AS (
	SELECT COUNT(DISTINCT ul.api_key_id) active_api_keys
	FROM usage_logs ul
	LEFT JOIN groups g ON g.id = ul.group_id
	LEFT JOIN accounts a ON a.id = ul.account_id
	` + usageWhereWithAlias + `
)
SELECT ` + selectCols + ` FROM usage_totals, active_users, active_api_keys`, args
}

func buildBusinessGroupsQuery(current, previous service.BusinessAnalyticsFilter) (string, []any) {
	currentTableName := "business_usage_daily"
	currentDateColumn := "bucket_date"
	weightedGroupRateExpr := "SUM(b.avg_group_rate_multiplier * GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL) / NULLIF(SUM(GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL), 0)"
	weightedSourceGroupRateExpr := "SUM(avg_group_rate_multiplier * GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL) / NULLIF(SUM(GREATEST(revenue, 0.000000001)) FILTER (WHERE avg_group_rate_multiplier IS NOT NULL), 0)"
	currentWhere, args := buildBusinessDailyWhere(current, currentDateColumn)
	currentSource := currentTableName + " " + currentWhere
	if includesToday(current) {
		currentWhere, args = buildUsageLogsWhere(current)
		currentSource = `(
			SELECT
				COALESCE(ul.group_id, 0) AS group_id,
				COUNT(*) AS requests,
				COUNT(DISTINCT ul.user_id) AS active_users,
				COUNT(DISTINCT ul.api_key_id) AS active_api_keys,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens), 0) AS total_tokens,
				COALESCE(SUM(ul.actual_cost), 0) AS revenue,
				` + businessUsageChannelCostSumExpr + ` AS channel_cost,
				` + businessUsageChannelGrossProfitSumExpr + ` AS gross_profit,
				SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL)
					/ NULLIF(SUM(GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL), 0) AS avg_group_rate_multiplier
			FROM usage_logs ul
			LEFT JOIN groups g ON g.id = ul.group_id
			LEFT JOIN accounts a ON a.id = ul.account_id
			` + currentWhere + `
			GROUP BY 1
		) current_usage`
	}
	previousWhere, prevArgs := buildBusinessDailyWhereFrom(previous, "p.bucket_date", len(args)+1)
	args = append(args, prevArgs...)
	if !includesToday(current) {
		currentAggregateWhere, _ := buildBusinessDailyWhere(current, "b."+currentDateColumn)
		currentDailyWhere, _ := buildBusinessDailyWhere(current, "b.bucket_date")
		currentUsageWhere, _ := buildUsageLogsWhere(current)
		return `
WITH current_usage AS (
	SELECT group_id, SUM(requests) requests, SUM(total_tokens) total_tokens, SUM(revenue) revenue, SUM(channel_cost) channel_cost, SUM(gross_profit) gross_profit, ` + weightedGroupRateExpr + ` avg_group_rate_multiplier
	FROM ` + currentTableName + ` b ` + currentAggregateWhere + ` GROUP BY group_id
), active_users AS (
	SELECT bu.group_id, COUNT(DISTINCT bu.user_id) active_users
	FROM business_usage_daily_users bu
	JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id
	` + currentDailyWhere + `
	GROUP BY bu.group_id
), active_api_keys AS (
	SELECT COALESCE(ul.group_id, 0) AS group_id, COUNT(DISTINCT ul.api_key_id) active_api_keys
	FROM usage_logs ul
	LEFT JOIN groups g ON g.id = ul.group_id
	LEFT JOIN accounts a ON a.id = ul.account_id
	` + currentUsageWhere + `
	GROUP BY COALESCE(ul.group_id, 0)
), previous_period AS (
	SELECT group_id, SUM(revenue) previous_revenue, SUM(gross_profit) previous_gross_profit
	FROM business_usage_daily p ` + previousWhere + ` GROUP BY group_id
)
SELECT
	cp.group_id,
	COALESCE(g.name, ''),
	COALESCE(g.platform, ''),
	g.rate_multiplier,
	cp.avg_group_rate_multiplier,
	COALESCE(cp.requests, 0),
	COALESCE(au.active_users, 0),
	COALESCE(aak.active_api_keys, 0),
	COALESCE(cp.total_tokens, 0),
	COALESCE(cp.revenue, 0),
	COALESCE(cp.channel_cost, 0),
	COALESCE(cp.gross_profit, 0),
	COALESCE(pp.previous_revenue, 0),
	COALESCE(pp.previous_gross_profit, 0)
FROM current_usage cp
LEFT JOIN active_users au ON au.group_id = cp.group_id
LEFT JOIN active_api_keys aak ON aak.group_id = cp.group_id
LEFT JOIN previous_period pp ON pp.group_id = cp.group_id
LEFT JOIN groups g ON g.id = cp.group_id
ORDER BY cp.revenue DESC, cp.group_id`, args
	}
	return `
WITH current_period AS (
	SELECT group_id, SUM(requests) requests, SUM(active_users) active_users, SUM(active_api_keys) active_api_keys, SUM(total_tokens) total_tokens, SUM(revenue) revenue, SUM(channel_cost) channel_cost, SUM(gross_profit) gross_profit, ` + weightedSourceGroupRateExpr + ` avg_group_rate_multiplier
	FROM ` + currentSource + ` GROUP BY group_id
), previous_period AS (
	SELECT group_id, SUM(revenue) previous_revenue, SUM(gross_profit) previous_gross_profit
	FROM business_usage_daily p ` + previousWhere + ` GROUP BY group_id
)
SELECT
	cp.group_id,
	COALESCE(g.name, ''),
	COALESCE(g.platform, ''),
	g.rate_multiplier,
	cp.avg_group_rate_multiplier,
	COALESCE(cp.requests, 0),
	COALESCE(cp.active_users, 0),
	COALESCE(cp.active_api_keys, 0),
	COALESCE(cp.total_tokens, 0),
	COALESCE(cp.revenue, 0),
	COALESCE(cp.channel_cost, 0),
	COALESCE(cp.gross_profit, 0),
	COALESCE(pp.previous_revenue, 0),
	COALESCE(pp.previous_gross_profit, 0)
FROM current_period cp
LEFT JOIN previous_period pp ON pp.group_id = cp.group_id
LEFT JOIN groups g ON g.id = cp.group_id
ORDER BY cp.revenue DESC, cp.group_id`, args
}

func buildBusinessChannelsQuery(filter service.BusinessAnalyticsFilter) (string, []any) {
	if includesToday(filter) {
		where, args := buildUsageLogsWhere(filter)
		return `
SELECT
	COALESCE(ul.account_id, 0),
	COALESCE(a.name, ''),
	COALESCE(MAX(ul.channel_id), 0),
	COALESCE(MAX(` + usageLogEffectivePlatformExpr + `), COALESCE(a.platform, '')),
	COALESCE(a.status, ''),
	a.channel_price,
	COALESCE(a.extra->>'balance_status', ''),
	SUM(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL)
		/ NULLIF(COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL), 0),
	COUNT(*),
	COUNT(DISTINCT ul.user_id),
	COUNT(DISTINCT ul.api_key_id),
	COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens), 0),
	COALESCE(SUM(ul.actual_cost), 0),
	` + businessUsageChannelCostSumExpr + `,
	` + businessUsageChannelGrossProfitSumExpr + `,
	COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL)
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
` + where + `
	GROUP BY COALESCE(ul.account_id, 0), a.name, a.platform, a.status, a.channel_price, a.extra
	ORDER BY SUM(ul.actual_cost) DESC, COALESCE(ul.account_id, 0)`, args
	}
	tableName := "business_usage_daily"
	dateColumn := "bucket_date"
	aggregateWhere, args := buildBusinessDailyWhere(filter, "b."+dateColumn)
	dailyWhere, _ := buildBusinessDailyWhere(filter, "b.bucket_date")
	usageWhere, _ := buildUsageLogsWhere(filter)
	weightedChannelPriceExpr := "SUM(b.avg_channel_price * GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL) / NULLIF(SUM(GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL), 0)"
	return `
WITH account_usage AS (
	SELECT
		b.account_id,
		COALESCE(MAX(b.channel_id), 0) channel_id,
		COALESCE(MAX(NULLIF(b.platform, '')), '') platform,
		COALESCE(SUM(b.requests), 0) requests,
		COALESCE(SUM(b.total_tokens), 0) total_tokens,
		COALESCE(SUM(b.revenue), 0) revenue,
		COALESCE(SUM(b.channel_cost), 0) channel_cost,
		COALESCE(SUM(b.gross_profit), 0) gross_profit,
		` + weightedChannelPriceExpr + ` avg_channel_price,
		COALESCE(SUM(b.missing_channel_price_records), 0) missing_channel_price_records
	FROM ` + tableName + ` b
	` + aggregateWhere + `
	GROUP BY b.account_id
), active_users AS (
	SELECT bu.account_id, COUNT(DISTINCT bu.user_id) active_users
	FROM business_usage_daily_users bu
	JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id
	` + dailyWhere + `
	GROUP BY bu.account_id
), active_api_keys AS (
	SELECT COALESCE(ul.account_id, 0) AS account_id, COUNT(DISTINCT ul.api_key_id) active_api_keys
	FROM usage_logs ul
	LEFT JOIN groups g ON g.id = ul.group_id
	LEFT JOIN accounts a ON a.id = ul.account_id
	` + usageWhere + `
	GROUP BY COALESCE(ul.account_id, 0)
)
SELECT
	au.account_id,
	COALESCE(a.name, ''),
	COALESCE(au.channel_id, 0),
	COALESCE(NULLIF(au.platform, ''), COALESCE(a.platform, '')),
	COALESCE(a.status, ''),
	a.channel_price,
	COALESCE(a.extra->>'balance_status', ''),
	au.avg_channel_price,
	COALESCE(au.requests, 0),
	COALESCE(active_users.active_users, 0),
	COALESCE(active_api_keys.active_api_keys, 0),
	COALESCE(au.total_tokens, 0),
	COALESCE(au.revenue, 0),
	COALESCE(au.channel_cost, 0),
	COALESCE(au.gross_profit, 0),
	COALESCE(au.missing_channel_price_records, 0)
FROM account_usage au
LEFT JOIN active_users ON active_users.account_id = au.account_id
LEFT JOIN active_api_keys ON active_api_keys.account_id = au.account_id
LEFT JOIN accounts a ON a.id = au.account_id
ORDER BY au.revenue DESC, au.account_id`, args
}

func buildBusinessDailyWhere(filter service.BusinessAnalyticsFilter, dateColumn string) (string, []any) {
	return buildBusinessDailyWhereFrom(filter, dateColumn, 1)
}

func buildBusinessDailyWhereFrom(filter service.BusinessAnalyticsFilter, dateColumn string, startIndex int) (string, []any) {
	conditions := []string{fmt.Sprintf("%s >= $%d::date", dateColumn, startIndex), fmt.Sprintf("%s < $%d::date", dateColumn, startIndex+1)}
	args := []any{filter.StartDate, filter.EndDate}
	columnPrefix := ""
	if dot := strings.LastIndex(dateColumn, "."); dot >= 0 {
		columnPrefix = dateColumn[:dot+1]
	}
	if filter.GroupID > 0 {
		args = append(args, filter.GroupID)
		conditions = append(conditions, fmt.Sprintf("%sgroup_id = $%d", columnPrefix, startIndex+len(args)-1))
	}
	if filter.AccountID > 0 {
		args = append(args, filter.AccountID)
		conditions = append(conditions, fmt.Sprintf("%saccount_id = $%d", columnPrefix, startIndex+len(args)-1))
	}
	if filter.Platform != "" {
		args = append(args, filter.Platform)
		conditions = append(conditions, fmt.Sprintf("%splatform = $%d", columnPrefix, startIndex+len(args)-1))
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func buildUsageLogsWhere(filter service.BusinessAnalyticsFilter) (string, []any) {
	conditions := []string{"ul.created_at >= $1", "ul.created_at < $2"}
	args := []any{filter.StartDate, filter.EndDate}
	if filter.GroupID > 0 {
		args = append(args, filter.GroupID)
		conditions = append(conditions, fmt.Sprintf("ul.group_id = $%d", len(args)))
	}
	if filter.AccountID > 0 {
		args = append(args, filter.AccountID)
		conditions = append(conditions, fmt.Sprintf("ul.account_id = $%d", len(args)))
	}
	if filter.Platform != "" {
		args = append(args, filter.Platform)
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s, '') = $%d", usageLogEffectivePlatformExpr, len(args)))
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func buildUsageLogAggregateQuery(filter service.BusinessAnalyticsFilter, trend bool) (string, []any) {
	where, args := buildUsageLogsWhere(filter)
	if trend {
		bucketExpr := "ul.created_at::date"
		if isWeeklyGranularity(filter) {
			bucketExpr = "date_trunc('week', ul.created_at)::date"
		}
		return `
SELECT
	` + bucketExpr + `::text,
	COUNT(*),
	COUNT(DISTINCT ul.user_id),
	COALESCE(SUM(ul.actual_cost), 0),
	` + businessUsageChannelCostSumExpr + `,
	` + businessUsageChannelGrossProfitSumExpr + `
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
` + where + `
GROUP BY ` + bucketExpr + `
ORDER BY ` + bucketExpr, args
	}
	return `
SELECT
	COUNT(*),
	COUNT(DISTINCT ul.user_id),
	COUNT(DISTINCT ul.api_key_id),
	COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens + ul.cache_creation_5m_tokens + ul.cache_creation_1h_tokens), 0),
	COALESCE(SUM(ul.actual_cost), 0),
	` + businessUsageChannelCostSumExpr + `,
	` + businessUsageChannelGrossProfitSumExpr + `,
	COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL)
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
LEFT JOIN accounts a ON a.id = ul.account_id
` + where, args
}

func isWeeklyGranularity(filter service.BusinessAnalyticsFilter) bool {
	return filter.Granularity == "week"
}

func includesToday(filter service.BusinessAnalyticsFilter) bool {
	now := time.Now().In(filter.EndDate.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, filter.EndDate.Location())
	return filter.EndDate.After(today)
}
