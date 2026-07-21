package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"golang.org/x/sync/errgroup"
)

// GetUserStatsAggregated returns aggregated usage statistics for a user using database-level aggregation
func (r *usageLogRepository) GetUserStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(COALESCE(duration_ms, 0)), 0) as avg_duration_ms
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`

	var stats usagestats.UsageStats
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{userID, startTime, endTime},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheTokens,
		&stats.TotalCacheCreationTokens,
		&stats.TotalCacheReadTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
	); err != nil {
		return nil, err
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	return &stats, nil
}

// GetAPIKeyStatsAggregated returns aggregated usage statistics for an API key using database-level aggregation
func (r *usageLogRepository) GetAPIKeyStatsAggregated(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(COALESCE(duration_ms, 0)), 0) as avg_duration_ms
		FROM usage_logs
		WHERE api_key_id = $1 AND created_at >= $2 AND created_at < $3
	`

	var stats usagestats.UsageStats
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{apiKeyID, startTime, endTime},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheTokens,
		&stats.TotalCacheCreationTokens,
		&stats.TotalCacheReadTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
	); err != nil {
		return nil, err
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	return &stats, nil
}

// GetAccountStatsAggregated 使用 SQL 聚合统计账号使用数据
//
// 性能优化说明：
// 原实现先查询所有日志记录，再在应用层循环计算统计值：
// 1. 需要传输大量数据到应用层
// 2. 应用层循环计算增加 CPU 和内存开销
//
// 新实现使用 SQL 聚合函数：
// 1. 在数据库层完成 COUNT/SUM/AVG 计算
// 2. 只返回单行聚合结果，大幅减少数据传输量
// 3. 利用数据库索引优化聚合查询性能
func (r *usageLogRepository) GetAccountStatsAggregated(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(COALESCE(duration_ms, 0)), 0) as avg_duration_ms
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
	`

	var stats usagestats.UsageStats
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{accountID, startTime, endTime},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheTokens,
		&stats.TotalCacheCreationTokens,
		&stats.TotalCacheReadTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
	); err != nil {
		return nil, err
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	return &stats, nil
}

// GetModelStatsAggregated 使用 SQL 聚合统计模型使用数据
// 性能优化：数据库层聚合计算，避免应用层循环统计
func (r *usageLogRepository) GetModelStatsAggregated(ctx context.Context, modelName string, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(COALESCE(duration_ms, 0)), 0) as avg_duration_ms
		FROM usage_logs
		WHERE %s = $1 AND created_at >= $2 AND created_at < $3
	`, rawUsageLogModelColumn)

	var stats usagestats.UsageStats
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{modelName, startTime, endTime},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheTokens,
		&stats.TotalCacheCreationTokens,
		&stats.TotalCacheReadTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
	); err != nil {
		return nil, err
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	return &stats, nil
}

// GetDailyStatsAggregated 使用 SQL 聚合统计用户的每日使用数据
// 性能优化：使用 GROUP BY 在数据库层按日期分组聚合，避免应用层循环分组统计
func (r *usageLogRepository) GetDailyStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (result []map[string]any, err error) {
	tzName := resolveUsageStatsTimezone()
	query := `
		SELECT
			-- 使用应用时区分组，避免数据库会话时区导致日边界偏移。
			TO_CHAR(created_at AT TIME ZONE $4, 'YYYY-MM-DD') as date,
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(COALESCE(duration_ms, 0)), 0) as avg_duration_ms
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1
		ORDER BY 1
	`

	rows, err := r.sql.QueryContext(ctx, query, userID, startTime, endTime, tzName)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	result = make([]map[string]any, 0)
	for rows.Next() {
		var (
			date              string
			totalRequests     int64
			totalInputTokens  int64
			totalOutputTokens int64
			totalCacheTokens  int64
			totalCost         float64
			totalActualCost   float64
			avgDurationMs     float64
		)
		if err = rows.Scan(
			&date,
			&totalRequests,
			&totalInputTokens,
			&totalOutputTokens,
			&totalCacheTokens,
			&totalCost,
			&totalActualCost,
			&avgDurationMs,
		); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"date":                date,
			"total_requests":      totalRequests,
			"total_input_tokens":  totalInputTokens,
			"total_output_tokens": totalOutputTokens,
			"total_cache_tokens":  totalCacheTokens,
			"total_tokens":        totalInputTokens + totalOutputTokens + totalCacheTokens,
			"total_cost":          totalCost,
			"total_actual_cost":   totalActualCost,
			"average_duration_ms": avgDurationMs,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// resolveUsageStatsTimezone 获取用于 SQL 分组的时区名称。
// 优先使用应用初始化的时区，其次尝试读取 TZ 环境变量，最后回落为 UTC。
func resolveUsageStatsTimezone() string {
	tzName := timezone.Name()
	if tzName != "" && tzName != "Local" {
		return tzName
	}
	if envTZ := strings.TrimSpace(os.Getenv("TZ")); envTZ != "" {
		return envTZ
	}
	return "UTC"
}

// openAISchedulerOverviewRepository keeps control-console SQL separate from the
// general UsageLogRepository contract while reusing its established SQL patterns.
type openAISchedulerOverviewRepository struct {
	sql sqlExecutor
}

func NewOpenAISchedulerOverviewRepository(db *sql.DB) service.OpenAISchedulerOverviewRepository {
	return newOpenAISchedulerOverviewRepositoryWithSQL(db)
}

func newOpenAISchedulerOverviewRepositoryWithSQL(sqlq sqlExecutor) *openAISchedulerOverviewRepository {
	return &openAISchedulerOverviewRepository{sql: sqlq}
}

func (r *openAISchedulerOverviewRepository) GetOpenAISchedulerOverviewMetrics(ctx context.Context, params service.OpenAISchedulerOverviewParams) (service.OpenAISchedulerOverviewMetrics, error) {
	var metrics service.OpenAISchedulerOverviewMetrics
	if r == nil || r.sql == nil {
		return metrics, nil
	}
	if err := r.loadOpenAISchedulerOverviewSummary(ctx, params, &metrics); err != nil {
		return service.OpenAISchedulerOverviewMetrics{}, err
	}
	groups, err := r.loadOpenAISchedulerOverviewGroups(ctx, params)
	if err != nil {
		return service.OpenAISchedulerOverviewMetrics{}, err
	}
	trend, err := r.loadOpenAISchedulerOverviewTrend(ctx, params)
	if err != nil {
		return service.OpenAISchedulerOverviewMetrics{}, err
	}
	slowCauses, err := r.loadOpenAISchedulerOverviewSlowCauses(ctx, params)
	if err != nil {
		return service.OpenAISchedulerOverviewMetrics{}, err
	}
	metrics.Groups = groups
	metrics.Trend = trend
	metrics.SlowCauses = slowCauses
	return metrics, nil
}

func (r *openAISchedulerOverviewRepository) loadOpenAISchedulerOverviewSummary(ctx context.Context, params service.OpenAISchedulerOverviewParams, metrics *service.OpenAISchedulerOverviewMetrics) error {
	query, args := buildOpenAISchedulerOverviewSummaryQuery(params)
	var realCount, probeCount int64
	if err := scanSingleRow(ctx, r.sql, query, args,
		&metrics.E2EP50MS, &metrics.E2EP90MS, &metrics.SelectionP95MS, &realCount, &probeCount,
	); err != nil {
		return err
	}
	denominator := realCount + probeCount
	if denominator > 0 {
		metrics.ProbeRatio = float64(probeCount) / float64(denominator)
	}
	return nil
}

func buildOpenAISchedulerOverviewSummaryQuery(params service.OpenAISchedulerOverviewParams) (string, []any) {
	args := []any{params.StartTime, params.EndTime}
	probeGroupFilter := ""
	usageGroupFilter := ""
	if params.GroupID > 0 {
		args = append(args, params.GroupID)
		probeGroupFilter = fmt.Sprintf(" AND e.group_id = $%d", len(args))
		usageGroupFilter = fmt.Sprintf(" AND ul.group_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		WITH physical_probes AS (
			SELECT DISTINCT e.account_id, e.model, e.event_type, e.created_at
			FROM openai_auto_scheduler_score_events e
			JOIN groups pg ON pg.id = e.group_id
			WHERE e.created_at >= $1 AND e.created_at < $2
			  AND pg.deleted_at IS NULL
			  AND (e.event_type IN ('probe_success', 'probe_error')
			       OR (e.event_type IN ('slow', 'severe_slow') AND e.status_code IS NULL))%s
		), usage_metrics AS (
			SELECT
				COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY ul.e2e_first_token_ms), 0) AS e2e_p50,
				COALESCE(PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY ul.e2e_first_token_ms), 0) AS e2e_p90,
				COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY ul.routing_ms) FILTER (WHERE ul.routing_ms IS NOT NULL), 0) AS selection_p95,
				COUNT(*) AS real_count
			FROM usage_logs ul
			JOIN groups g ON g.id = ul.group_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.e2e_first_token_ms IS NOT NULL
			  AND g.platform = 'openai'
			  AND g.deleted_at IS NULL%s
		)
		SELECT e2e_p50, e2e_p90, selection_p95, real_count,
		       (SELECT COUNT(*) FROM physical_probes) AS probe_count
		FROM usage_metrics
	`, probeGroupFilter, usageGroupFilter)
	return query, args
}

func (r *openAISchedulerOverviewRepository) loadOpenAISchedulerOverviewGroups(ctx context.Context, params service.OpenAISchedulerOverviewParams) (result []service.OpenAISchedulerGroupSummary, err error) {
	args := []any{params.StartTime, params.EndTime, service.PlatformOpenAI}
	groupFilter := ""
	if params.GroupID > 0 {
		args = append(args, params.GroupID)
		groupFilter = fmt.Sprintf(" AND g.id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		WITH group_usage AS (
			SELECT ul.group_id,
			       PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY ul.e2e_first_token_ms) AS e2e_p90
			FROM usage_logs ul
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.e2e_first_token_ms IS NOT NULL
			GROUP BY ul.group_id
		), memberships AS (
			SELECT ag.group_id, COUNT(*) AS account_count
			FROM account_groups ag
			JOIN accounts a ON a.id = ag.account_id
			WHERE a.platform = $3 AND a.deleted_at IS NULL
			GROUP BY ag.group_id
		)
		SELECT g.id, g.name, g.openai_auto_scheduler_enabled,
		       COALESCE(m.account_count, 0) AS account_count,
		       COALESCE(gu.e2e_p90, 0) AS e2e_p90
		FROM groups g
		LEFT JOIN memberships m ON m.group_id = g.id
		LEFT JOIN group_usage gu ON gu.group_id = g.id
		WHERE g.platform = $3 AND g.deleted_at IS NULL%s
		ORDER BY g.sort_order ASC, g.id ASC
	`, groupFilter)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = make([]service.OpenAISchedulerGroupSummary, 0)
	for rows.Next() {
		var item service.OpenAISchedulerGroupSummary
		if err = rows.Scan(&item.ID, &item.Name, &item.Enabled, &item.AccountCount, &item.E2EP90MS); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func buildOpenAISchedulerOverviewTrendQuery(params service.OpenAISchedulerOverviewParams) (string, []any) {
	bucketExpression := "DATE_TRUNC('hour', ul.created_at)"
	if params.Bucket == 6*time.Hour {
		bucketExpression = "(DATE_TRUNC('day', ul.created_at AT TIME ZONE 'UTC') + FLOOR(EXTRACT(HOUR FROM ul.created_at AT TIME ZONE 'UTC') / 6) * INTERVAL '6 hours') AT TIME ZONE 'UTC'"
	}
	args := []any{params.StartTime, params.EndTime}
	groupFilter := ""
	if params.GroupID > 0 {
		args = append(args, params.GroupID)
		groupFilter = fmt.Sprintf(" AND ul.group_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT %s AS bucket,
		       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY ul.e2e_first_token_ms), 0) AS e2e_p50,
		       COALESCE(PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY ul.e2e_first_token_ms), 0) AS e2e_p90
		FROM usage_logs ul
		JOIN groups g ON g.id = ul.group_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND ul.e2e_first_token_ms IS NOT NULL
		  AND g.platform = 'openai'
		  AND g.deleted_at IS NULL%s
		GROUP BY bucket
		ORDER BY bucket ASC
	`, bucketExpression, groupFilter)
	return query, args
}

func (r *openAISchedulerOverviewRepository) loadOpenAISchedulerOverviewTrend(ctx context.Context, params service.OpenAISchedulerOverviewParams) (result []service.OpenAISchedulerTrendPoint, err error) {
	query, args := buildOpenAISchedulerOverviewTrendQuery(params)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = make([]service.OpenAISchedulerTrendPoint, 0)
	for rows.Next() {
		var point service.OpenAISchedulerTrendPoint
		if err = rows.Scan(&point.Bucket, &point.E2EP50MS, &point.E2EP90MS); err != nil {
			return nil, err
		}
		result = append(result, point)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *openAISchedulerOverviewRepository) loadOpenAISchedulerOverviewSlowCauses(ctx context.Context, params service.OpenAISchedulerOverviewParams) (result []service.OpenAISchedulerSlowCause, err error) {
	args := []any{params.StartTime, params.EndTime, params.SlowThresholdMS}
	groupFilter := ""
	if params.GroupID > 0 {
		args = append(args, params.GroupID)
		groupFilter = fmt.Sprintf(" AND ul.group_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		WITH contributions AS (
			SELECT COALESCE(ul.queue_ms, 0) AS queue_ms,
			       COALESCE(ul.retry_ms, 0) AS retry_ms,
			       GREATEST(ul.e2e_first_token_ms - COALESCE(ul.routing_ms, 0) - COALESCE(ul.queue_ms, 0) - COALESCE(ul.retry_ms, 0), 0) AS upstream_ms
			FROM usage_logs ul
			JOIN groups g ON g.id = ul.group_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.e2e_first_token_ms >= $3
			  AND g.platform = 'openai'
			  AND g.deleted_at IS NULL%s
		), classified AS (
			SELECT CASE
				WHEN queue_ms >= retry_ms AND queue_ms > upstream_ms THEN 'queue'
				WHEN retry_ms > queue_ms AND retry_ms > upstream_ms THEN 'retry'
				ELSE 'upstream_ttft'
			END AS reason
			FROM contributions
		)
		SELECT reason, COUNT(*) AS cause_count
		FROM classified
		GROUP BY reason
		ORDER BY cause_count DESC, reason ASC
	`, groupFilter)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = make([]service.OpenAISchedulerSlowCause, 0, 3)
	var total int64
	for rows.Next() {
		var cause service.OpenAISchedulerSlowCause
		if err = rows.Scan(&cause.Reason, &cause.Count); err != nil {
			return nil, err
		}
		total += cause.Count
		result = append(result, cause)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if total > 0 {
		for i := range result {
			result[i].Ratio = float64(result[i].Count) / float64(total)
		}
	}
	return result, nil
}

func buildOpenAISchedulerHealthQueries(params service.OpenAISchedulerHealthParams) (string, string, []any, []any) {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	conditions := []string{"a.platform = $1", "g.platform = $1", "a.deleted_at IS NULL", "g.deleted_at IS NULL"}
	args := []any{service.PlatformOpenAI}
	appendFilter := func(column string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if params.GroupID > 0 {
		appendFilter("ag.group_id", params.GroupID)
	}
	if value := strings.TrimSpace(params.State); value != "" {
		appendFilter("hs.state", value)
	}
	if value := strings.TrimSpace(params.ModelFamily); value != "" {
		appendFilter("hs.model_family", value)
	}
	if value := strings.TrimSpace(params.Endpoint); value != "" {
		appendFilter("hs.endpoint", value)
	}
	if value := strings.TrimSpace(params.Transport); value != "" {
		appendFilter("hs.transport", value)
	}
	fromWhere := `
		FROM account_groups ag
		JOIN accounts a ON a.id = ag.account_id
		JOIN groups g ON g.id = ag.group_id
		LEFT JOIN openai_scheduler_health_states hs ON hs.account_id = ag.account_id
		WHERE ` + strings.Join(conditions, " AND ")
	countQuery := "SELECT COUNT(*) " + fromWhere

	sortColumns := map[string]string{
		"account_id": "account_id", "predicted_ttft_ms": "predicted_ttft_ms", "error_rate": "error_rate",
		"real_sample_count": "real_sample_count", "probe_sample_count": "probe_sample_count",
		"snapshot_age_ms": "updated_at", "channel_price": "channel_price",
	}
	sortColumn := sortColumns[strings.ToLower(strings.TrimSpace(params.Sort))]
	if sortColumn == "" {
		sortColumn = "predicted_ttft_ms"
	}
	order := strings.ToUpper(strings.TrimSpace(params.Order))
	if order != "ASC" {
		order = "DESC"
	}
	if strings.EqualFold(params.Sort, "snapshot_age_ms") {
		if order == "ASC" {
			order = "DESC"
		} else {
			order = "ASC"
		}
	}
	rowsArgs := append([]any(nil), args...)
	rowsArgs = append(rowsArgs, pageSize, (page-1)*pageSize)
	limitPlaceholder := len(rowsArgs) - 1
	offsetPlaceholder := len(rowsArgs)
	rowsQuery := fmt.Sprintf(`
		WITH health_rows AS (
			SELECT a.id AS account_id, a.name AS account_name, ag.group_id, ag.priority AS group_priority,
			       g.status AS group_status, g.openai_auto_scheduler_enabled AS group_auto_scheduler_enabled,
			       a.status AS account_status, a.schedulable,
			       a.temp_unschedulable_until, COALESCE(a.temp_unschedulable_reason, '') AS temp_unschedulable_reason,
			       a.auto_pause_on_expired, a.expires_at AS account_expires_at,
			       a.overload_until, a.rate_limit_reset_at,
			       COALESCE(hs.model_family, '') AS model_family,
			       COALESCE(hs.endpoint, '') AS endpoint,
			       COALESCE(hs.transport, '') AS transport,
			       COALESCE(hs.state, '') AS state,
			       COALESCE(hs.predicted_ttft_ms, 0) AS predicted_ttft_ms,
			       COALESCE(hs.real_sample_count, 0) AS real_sample_count,
			       COALESCE(hs.probe_sample_count, 0) AS probe_sample_count,
			       COALESCE(hs.error_rate, 0) AS error_rate,
			       COALESCE(hs.rate_limited_rate, 0) AS rate_limited_rate,
			       COALESCE(hs.server_error_rate, 0) AS server_error_rate,
			       hs.cooldown_until, hs.expires_at, hs.last_real_at, hs.updated_at,
			       CASE WHEN a.load_factor > 0 THEN a.load_factor WHEN a.concurrency > 0 THEN a.concurrency ELSE 1 END AS load_capacity,
			       a.channel_price
			%s
		)
		SELECT account_id, account_name, group_id, group_priority, group_status, group_auto_scheduler_enabled,
		       account_status, schedulable,
		       temp_unschedulable_until, temp_unschedulable_reason,
		       auto_pause_on_expired, account_expires_at, overload_until, rate_limit_reset_at,
		       model_family, endpoint, transport, state,
		       predicted_ttft_ms, real_sample_count, probe_sample_count,
		       error_rate, rate_limited_rate, server_error_rate, cooldown_until, expires_at,
		       last_real_at, updated_at, load_capacity, channel_price
		FROM health_rows
		ORDER BY %s %s NULLS LAST, account_id ASC, group_id ASC, model_family ASC, endpoint ASC, transport ASC
		LIMIT $%d OFFSET $%d
	`, fromWhere, sortColumn, order, limitPlaceholder, offsetPlaceholder)
	return countQuery, rowsQuery, args, rowsArgs
}

func (r *openAISchedulerOverviewRepository) ListOpenAISchedulerHealth(ctx context.Context, params service.OpenAISchedulerHealthParams) (result []service.OpenAISchedulerHealthRecord, total int64, err error) {
	if r == nil || r.sql == nil {
		return []service.OpenAISchedulerHealthRecord{}, 0, nil
	}
	countQuery, rowsQuery, countArgs, rowsArgs := buildOpenAISchedulerHealthQueries(params)
	if err := scanSingleRow(ctx, r.sql, countQuery, countArgs, &total); err != nil {
		return nil, 0, err
	}
	rows, err := r.sql.QueryContext(ctx, rowsQuery, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
			total = 0
		}
	}()
	result = make([]service.OpenAISchedulerHealthRecord, 0)
	for rows.Next() {
		var item service.OpenAISchedulerHealthRecord
		var tempUnschedulableUntil, accountExpiresAt, overloadUntil, rateLimitResetAt sql.NullTime
		var cooldownUntil, expiresAt, lastRealAt, updatedAt sql.NullTime
		var channelPrice sql.NullFloat64
		if err = rows.Scan(
			&item.AccountID, &item.AccountName, &item.GroupID, &item.GroupPriority, &item.GroupStatus, &item.GroupAutoSchedulerEnabled,
			&item.AccountStatus, &item.Schedulable,
			&tempUnschedulableUntil, &item.TempUnschedulableReason,
			&item.AutoPauseOnExpired, &accountExpiresAt, &overloadUntil, &rateLimitResetAt,
			&item.ModelFamily, &item.Endpoint, &item.Transport,
			&item.State, &item.PredictedTTFTMS, &item.RealSampleCount,
			&item.ProbeSampleCount, &item.ErrorRate, &item.RateLimitedRate, &item.ServerErrorRate,
			&cooldownUntil, &expiresAt, &lastRealAt, &updatedAt, &item.LoadCapacity, &channelPrice,
		); err != nil {
			return nil, 0, err
		}
		if tempUnschedulableUntil.Valid {
			item.TempUnschedulableUntil = &tempUnschedulableUntil.Time
		}
		if accountExpiresAt.Valid {
			item.AccountExpiresAt = &accountExpiresAt.Time
		}
		if overloadUntil.Valid {
			item.OverloadUntil = &overloadUntil.Time
		}
		if rateLimitResetAt.Valid {
			item.RateLimitResetAt = &rateLimitResetAt.Time
		}
		if cooldownUntil.Valid {
			item.CooldownUntil = &cooldownUntil.Time
		}
		if expiresAt.Valid {
			item.ExpiresAt = expiresAt.Time
		}
		if lastRealAt.Valid {
			item.LastRealAt = &lastRealAt.Time
		}
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.Time
		}
		if channelPrice.Valid {
			price := channelPrice.Float64
			item.ChannelPrice = &price
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (r *openAISchedulerOverviewRepository) ListOpenAISchedulerRankingActual(
	ctx context.Context,
	params service.OpenAISchedulerRankingActualParams,
) (result []service.OpenAISchedulerRankingActual, err error) {
	if r == nil || r.sql == nil {
		return []service.OpenAISchedulerRankingActual{}, nil
	}
	conditions := []string{"group_id = $1", "created_at >= $2", "created_at < $3"}
	args := []any{params.GroupID, params.StartTime, params.EndTime}
	appendFilter := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	appendFilter("model_family", strings.ToLower(strings.TrimSpace(params.ModelFamily)))
	appendFilter("endpoint", strings.ToLower(strings.TrimSpace(params.Endpoint)))
	appendFilter("transport", strings.ToLower(strings.TrimSpace(params.Transport)))
	query := fmt.Sprintf(`
		WITH normalized AS (
			SELECT account_id,
			       LOWER(COALESCE(NULLIF(upstream_model, ''), NULLIF(model, ''), '')) AS model_family,
			       CASE
			           WHEN LOWER(COALESCE(upstream_endpoint, '')) LIKE '%%chat%%completions%%' THEN 'chat_completions'
			           WHEN LOWER(COALESCE(upstream_endpoint, '')) LIKE '%%embeddings%%' THEN 'embeddings'
			           WHEN LOWER(COALESCE(upstream_endpoint, '')) LIKE '%%images%%edits%%' THEN 'images_edits'
			           WHEN LOWER(COALESCE(upstream_endpoint, '')) LIKE '%%images%%generations%%' THEN 'images_generations'
			           ELSE 'responses'
			       END AS endpoint,
			       CASE WHEN openai_ws_mode THEN 'responses_websockets_v2' ELSE 'http_sse' END AS transport,
			       created_at, e2e_first_token_ms, first_token_ms,
			       COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1) AS estimated_cost,
			       group_id
			FROM usage_logs
			WHERE group_id = $1 AND created_at >= $2 AND created_at < $3
		)
		SELECT account_id, model_family, endpoint, transport,
		       COUNT(*) AS request_count,
		       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY COALESCE(e2e_first_token_ms, first_token_ms))
		           FILTER (WHERE COALESCE(e2e_first_token_ms, first_token_ms) IS NOT NULL), 0) AS ttft_p50,
		       COALESCE(PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY COALESCE(e2e_first_token_ms, first_token_ms))
		           FILTER (WHERE COALESCE(e2e_first_token_ms, first_token_ms) IS NOT NULL), 0) AS ttft_p90,
		       COALESCE(SUM(estimated_cost), 0) AS estimated_cost
		FROM normalized
		WHERE %s
		GROUP BY account_id, model_family, endpoint, transport
		ORDER BY model_family, endpoint, transport, account_id
	`, strings.Join(conditions, " AND "))
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = make([]service.OpenAISchedulerRankingActual, 0)
	for rows.Next() {
		var item service.OpenAISchedulerRankingActual
		if err = rows.Scan(
			&item.Key.AccountID, &item.Key.ModelFamily, &item.Key.Endpoint, &item.Key.Transport,
			&item.RequestCount, &item.TTFTP50MS, &item.TTFTP90MS, &item.EstimatedCost,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAccountTodayStats 获取账号今日统计
func (r *usageLogRepository) GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error) {
	today := timezone.Today()

	query := `
		SELECT
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(total_cost), 0) as standard_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2
	`

	stats := &usagestats.AccountStats{}
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{accountID, today},
		&stats.Requests,
		&stats.Tokens,
		&stats.Cost,
		&stats.StandardCost,
		&stats.UserCost,
	); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetAccountWindowStats 获取账号时间窗口内的统计
func (r *usageLogRepository) GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error) {
	query := `
		SELECT
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(total_cost), 0) as standard_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2
	`

	stats := &usagestats.AccountStats{}
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{accountID, startTime},
		&stats.Requests,
		&stats.Tokens,
		&stats.Cost,
		&stats.StandardCost,
		&stats.UserCost,
	); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetAccountWindowStatsBatch 批量获取同一窗口起点下多个账号的统计数据。
// 返回 map[accountID]*AccountStats，未命中的账号会返回零值统计，便于上层直接复用。
func (r *usageLogRepository) GetAccountWindowStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	result := make(map[int64]*usagestats.AccountStats, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT
			account_id,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(total_cost), 0) as standard_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE account_id = ANY($1) AND created_at >= $2
		GROUP BY account_id
	`
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(accountIDs), startTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var accountID int64
		stats := &usagestats.AccountStats{}
		if err := rows.Scan(
			&accountID,
			&stats.Requests,
			&stats.Tokens,
			&stats.Cost,
			&stats.StandardCost,
			&stats.UserCost,
		); err != nil {
			return nil, err
		}
		result[accountID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, accountID := range accountIDs {
		if _, ok := result[accountID]; !ok {
			result[accountID] = &usagestats.AccountStats{}
		}
	}
	return result, nil
}

// GetGroupUserDailyStatsBatch aggregates account-management-compatible stats by user for one group and one window.
// Unmatched users are returned with zero-value stats so callers can index the result without extra checks.
func (r *usageLogRepository) GetGroupUserDailyStatsBatch(ctx context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	result := make(map[int64]*usagestats.AccountStats, len(userIDs))
	if groupID <= 0 || len(userIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT
			user_id,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(total_cost), 0) as standard_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE group_id = $1
		  AND user_id = ANY($2)
		  AND created_at >= $3
		  AND created_at < $4
		GROUP BY user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, groupID, pq.Array(userIDs), startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		stats := &usagestats.AccountStats{}
		if err := rows.Scan(
			&userID,
			&stats.Requests,
			&stats.Tokens,
			&stats.Cost,
			&stats.StandardCost,
			&stats.UserCost,
		); err != nil {
			return nil, err
		}
		result[userID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, userID := range userIDs {
		if _, ok := result[userID]; !ok {
			result[userID] = &usagestats.AccountStats{}
		}
	}
	return result, nil
}

// GetGeminiUsageTotalsBatch 批量聚合 Gemini 账号在窗口内的 Pro/Flash 请求与用量。
// 模型分类规则与 service.geminiModelClassFromName 一致：model 包含 flash/lite 视为 flash，其余视为 pro。
func (r *usageLogRepository) GetGeminiUsageTotalsBatch(ctx context.Context, accountIDs []int64, startTime, endTime time.Time) (map[int64]service.GeminiUsageTotals, error) {
	result := make(map[int64]service.GeminiUsageTotals, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT
			account_id,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN 1 ELSE 0 END), 0) AS flash_requests,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN 0 ELSE 1 END), 0) AS pro_requests,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN (input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens) ELSE 0 END), 0) AS flash_tokens,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN 0 ELSE (input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens) END), 0) AS pro_tokens,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN actual_cost ELSE 0 END), 0) AS flash_cost,
			COALESCE(SUM(CASE WHEN LOWER(COALESCE(model, '')) LIKE '%flash%' OR LOWER(COALESCE(model, '')) LIKE '%lite%' THEN 0 ELSE actual_cost END), 0) AS pro_cost
		FROM usage_logs
		WHERE account_id = ANY($1) AND created_at >= $2 AND created_at < $3
		GROUP BY account_id
	`
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(accountIDs), startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var accountID int64
		var totals service.GeminiUsageTotals
		if err := rows.Scan(
			&accountID,
			&totals.FlashRequests,
			&totals.ProRequests,
			&totals.FlashTokens,
			&totals.ProTokens,
			&totals.FlashCost,
			&totals.ProCost,
		); err != nil {
			return nil, err
		}
		result[accountID] = totals
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, accountID := range accountIDs {
		if _, ok := result[accountID]; !ok {
			result[accountID] = service.GeminiUsageTotals{}
		}
	}
	return result, nil
}

// UsageStats represents usage statistics
type UsageStats = usagestats.UsageStats

// BatchUserUsageStats represents usage stats for a single user
type BatchUserUsageStats = usagestats.BatchUserUsageStats

// PlatformUsage represents per-platform usage breakdown
type PlatformUsage = usagestats.PlatformUsage

func normalizePositiveInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// GetBatchUserUsageStats gets today and total actual_cost for multiple users within a time range.
// If startTime is zero, defaults to 30 days ago.
func (r *usageLogRepository) GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*BatchUserUsageStats, error) {
	result := make(map[int64]*BatchUserUsageStats)
	normalizedUserIDs := normalizePositiveInt64IDs(userIDs)
	if len(normalizedUserIDs) == 0 {
		return result, nil
	}

	// 默认最近 30 天
	if startTime.IsZero() {
		startTime = time.Now().AddDate(0, 0, -30)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}

	for _, id := range normalizedUserIDs {
		result[id] = &BatchUserUsageStats{UserID: id}
	}

	// GROUP BY (user_id, effective_platform) 一次查询同时得到总值与按平台拆分。
	// 应用层把同一 user_id 的多行累加为总值，并把非空 platform 行收集到 ByPlatform。
	query := `
		SELECT
			ul.user_id,
			` + usageLogEffectivePlatformExpr + ` as platform,
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $2 AND ul.created_at < $3), 0) as total_cost,
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $4), 0) as today_cost
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id
		LEFT JOIN accounts a ON a.id = ul.account_id
		WHERE ul.user_id = ANY($1)
		  AND ul.created_at >= LEAST($2, $4)
		  AND ` + usageLogSuccessFilterUL + `
		GROUP BY ul.user_id, ` + usageLogEffectivePlatformExpr + `
	`
	today := timezone.Today()
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(normalizedUserIDs), startTime, endTime, today)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userID int64
		var platform sql.NullString
		var total float64
		var todayTotal float64
		if err := rows.Scan(&userID, &platform, &total, &todayTotal); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats, ok := result[userID]
		if !ok {
			continue
		}
		stats.TotalActualCost += total
		stats.TodayActualCost += todayTotal
		if platform.Valid && platform.String != "" {
			stats.ByPlatform = append(stats.ByPlatform, PlatformUsage{
				Platform:        platform.String,
				TotalActualCost: total,
				TodayActualCost: todayTotal,
			})
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// BatchAPIKeyUsageStats represents usage stats for a single API key
type BatchAPIKeyUsageStats = usagestats.BatchAPIKeyUsageStats

// GetBatchAPIKeyUsageStats gets today and total actual_cost for multiple API keys within a time range.
// If startTime is zero, defaults to 30 days ago.
func (r *usageLogRepository) GetBatchAPIKeyUsageStats(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time) (map[int64]*BatchAPIKeyUsageStats, error) {
	result := make(map[int64]*BatchAPIKeyUsageStats)
	normalizedAPIKeyIDs := normalizePositiveInt64IDs(apiKeyIDs)
	if len(normalizedAPIKeyIDs) == 0 {
		return result, nil
	}

	// 默认最近 30 天
	if startTime.IsZero() {
		startTime = time.Now().AddDate(0, 0, -30)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}

	for _, id := range normalizedAPIKeyIDs {
		result[id] = &BatchAPIKeyUsageStats{APIKeyID: id}
	}

	query := `
		SELECT
			api_key_id,
			COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $2 AND created_at < $3), 0) as total_cost,
			COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $4), 0) as today_cost
		FROM usage_logs
		WHERE api_key_id = ANY($1)
		  AND created_at >= LEAST($2, $4)
		GROUP BY api_key_id
	`
	today := timezone.Today()
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(normalizedAPIKeyIDs), startTime, endTime, today)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var apiKeyID int64
		var total float64
		var todayTotal float64
		if err := rows.Scan(&apiKeyID, &total, &todayTotal); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if stats, ok := result[apiKeyID]; ok {
			stats.TotalActualCost = total
			stats.TodayActualCost = todayTotal
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// resolveEndpointColumn maps endpoint type to the corresponding DB column name.
func resolveEndpointColumn(endpointType string) string {
	switch endpointType {
	case "upstream":
		return "ul.upstream_endpoint"
	case "path":
		return "ul.inbound_endpoint || ' -> ' || ul.upstream_endpoint"
	default:
		return "ul.inbound_endpoint"
	}
}

// GetGlobalStats gets usage statistics for all users within a time range
func (r *usageLogRepository) GetGlobalStats(ctx context.Context, startTime, endTime time.Time) (*UsageStats, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
	`

	stats := &UsageStats{}
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{startTime, endTime},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
	); err != nil {
		return nil, err
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	return stats, nil
}

// GetStatsWithFilters gets usage statistics with optional filters
func (r *usageLogRepository) GetStatsWithFilters(ctx context.Context, filters UsageLogFilters) (*UsageStats, error) {
	conditions := make([]string, 0, 9)
	args := make([]any, 0, 9)

	if filters.UserID > 0 {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", len(args)+1))
		args = append(args, filters.UserID)
	}
	if filters.APIKeyID > 0 {
		conditions = append(conditions, fmt.Sprintf("api_key_id = $%d", len(args)+1))
		args = append(args, filters.APIKeyID)
	}
	if filters.AccountID > 0 {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", len(args)+1))
		args = append(args, filters.AccountID)
	}
	if filters.GroupID > 0 {
		conditions = append(conditions, fmt.Sprintf("group_id = $%d", len(args)+1))
		args = append(args, filters.GroupID)
	}
	conditions, args = appendUsageLogModelWhereCondition(conditions, args, filters.Model, filters.ModelFilterSource)
	conditions, args = appendRequestTypeOrStreamWhereCondition(conditions, args, filters.RequestType, filters.Stream)
	if filters.BillingType != nil {
		conditions = append(conditions, fmt.Sprintf("billing_type = $%d", len(args)+1))
		args = append(args, int16(*filters.BillingType))
	}
	conditions, args = appendUsageLogBillingModeWhereCondition(conditions, args, filters.BillingMode)
	if filters.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, *filters.EndTime)
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(actual_cost), 0) as total_actual_cost,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as total_account_cost,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms
		FROM usage_logs
		%s
	`, buildWhere(conditions))

	stats := &UsageStats{}
	var totalAccountCost float64

	start := time.Unix(0, 0).UTC()
	if filters.StartTime != nil {
		start = *filters.StartTime
	}
	end := time.Now().UTC()
	if filters.EndTime != nil {
		end = *filters.EndTime
	}

	var endpoints, upstreamEndpoints, endpointPaths []EndpointStat

	// 汇总查询:失败即致命。
	runSummary := func(c context.Context) error {
		return scanSingleRow(
			c, r.sql, query, args,
			&stats.TotalRequests,
			&stats.TotalInputTokens,
			&stats.TotalOutputTokens,
			&stats.TotalCacheTokens,
			&stats.TotalCacheCreationTokens,
			&stats.TotalCacheReadTokens,
			&stats.TotalCost,
			&stats.TotalActualCost,
			&totalAccountCost,
			&stats.AverageDurationMs,
		)
	}
	// endpoint 明细:best-effort(失败 log + 返空),不致命。
	runEndpoints := func(c context.Context) {
		res, err := r.getEndpointStatsByColumnWithFilters(c, "inbound_endpoint", start, end, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.Model, filters.ModelFilterSource, filters.RequestType, filters.Stream, filters.BillingType, filters.BillingMode)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				logger.LegacyPrintf("repository.usage_log", "GetEndpointStatsWithFilters failed in GetStatsWithFilters: %v", err)
			}
			res = []EndpointStat{}
		}
		endpoints = res
	}
	runUpstream := func(c context.Context) {
		res, err := r.getEndpointStatsByColumnWithFilters(c, "upstream_endpoint", start, end, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.Model, filters.ModelFilterSource, filters.RequestType, filters.Stream, filters.BillingType, filters.BillingMode)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				logger.LegacyPrintf("repository.usage_log", "GetUpstreamEndpointStatsWithFilters failed in GetStatsWithFilters: %v", err)
			}
			res = []EndpointStat{}
		}
		upstreamEndpoints = res
	}
	runPaths := func(c context.Context) {
		res, err := r.getEndpointPathStatsWithFilters(c, start, end, filters.UserID, filters.APIKeyID, filters.AccountID, filters.GroupID, filters.Model, filters.ModelFilterSource, filters.RequestType, filters.Stream, filters.BillingType, filters.BillingMode)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				logger.LegacyPrintf("repository.usage_log", "getEndpointPathStatsWithFilters failed in GetStatsWithFilters: %v", err)
			}
			res = []EndpointStat{}
		}
		endpointPaths = res
	}

	if r.db != nil {
		// 生产路径:r.sql 是 *sql.DB 连接池,可并发。4 条查询并行,延迟取最大值。
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() error { return runSummary(gctx) })
		g.Go(func() error { runEndpoints(gctx); return nil })
		g.Go(func() error { runUpstream(gctx); return nil })
		g.Go(func() error { runPaths(gctx); return nil })
		if err := g.Wait(); err != nil {
			return nil, err
		}
	} else {
		// 事务路径(ent.Tx 不能并发查询):顺序执行,行为与重构前一致。
		if err := runSummary(ctx); err != nil {
			return nil, err
		}
		runEndpoints(ctx)
		runUpstream(ctx)
		runPaths(ctx)
	}

	stats.TotalAccountCost = &totalAccountCost
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	stats.Endpoints = endpoints
	stats.UpstreamEndpoints = upstreamEndpoints
	stats.EndpointPaths = endpointPaths

	return stats, nil
}

// AccountUsageHistory represents daily usage history for an account
type AccountUsageHistory = usagestats.AccountUsageHistory

// AccountUsageSummary represents summary statistics for an account
type AccountUsageSummary = usagestats.AccountUsageSummary

// AccountUsageStatsResponse represents the full usage statistics response for an account
type AccountUsageStatsResponse = usagestats.AccountUsageStatsResponse

// EndpointStat represents endpoint usage statistics row.
type EndpointStat = usagestats.EndpointStat

func (r *usageLogRepository) getEndpointStatsByColumnWithFilters(ctx context.Context, endpointColumn string, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, modelSource string, requestType *int16, stream *bool, billingType *int8, billingMode string) (results []EndpointStat, err error) {
	actualCostExpr := "COALESCE(SUM(actual_cost), 0) as actual_cost"
	if accountID > 0 && userID == 0 && apiKeyID == 0 {
		actualCostExpr = "COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as actual_cost"
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(TRIM(%s), ''), 'unknown') AS endpoint,
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(total_cost), 0) as cost,
			%s
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
	`, endpointColumn, actualCostExpr)

	args := []any{startTime, endTime}
	if userID > 0 {
		query += fmt.Sprintf(" AND user_id = $%d", len(args)+1)
		args = append(args, userID)
	}
	if apiKeyID > 0 {
		query += fmt.Sprintf(" AND api_key_id = $%d", len(args)+1)
		args = append(args, apiKeyID)
	}
	if accountID > 0 {
		query += fmt.Sprintf(" AND account_id = $%d", len(args)+1)
		args = append(args, accountID)
	}
	if groupID > 0 {
		query += fmt.Sprintf(" AND group_id = $%d", len(args)+1)
		args = append(args, groupID)
	}
	query, args = appendUsageLogModelQueryFilter(query, args, model, modelSource)
	query, args = appendRequestTypeOrStreamQueryFilter(query, args, requestType, stream)
	if billingType != nil {
		query += fmt.Sprintf(" AND billing_type = $%d", len(args)+1)
		args = append(args, int16(*billingType))
	}
	query, args = appendUsageLogBillingModeQueryFilter(query, args, billingMode, "")
	query += " GROUP BY endpoint ORDER BY requests DESC"

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]EndpointStat, 0)
	for rows.Next() {
		var row EndpointStat
		if err := rows.Scan(&row.Endpoint, &row.Requests, &row.TotalTokens, &row.Cost, &row.ActualCost); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *usageLogRepository) getEndpointPathStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, modelSource string, requestType *int16, stream *bool, billingType *int8, billingMode string) (results []EndpointStat, err error) {
	actualCostExpr := "COALESCE(SUM(actual_cost), 0) as actual_cost"
	if accountID > 0 && userID == 0 && apiKeyID == 0 {
		actualCostExpr = "COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as actual_cost"
	}

	query := fmt.Sprintf(`
		SELECT
			CONCAT(
				COALESCE(NULLIF(TRIM(inbound_endpoint), ''), 'unknown'),
				' -> ',
				COALESCE(NULLIF(TRIM(upstream_endpoint), ''), 'unknown')
			) AS endpoint,
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(total_cost), 0) as cost,
			%s
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
	`, actualCostExpr)

	args := []any{startTime, endTime}
	if userID > 0 {
		query += fmt.Sprintf(" AND user_id = $%d", len(args)+1)
		args = append(args, userID)
	}
	if apiKeyID > 0 {
		query += fmt.Sprintf(" AND api_key_id = $%d", len(args)+1)
		args = append(args, apiKeyID)
	}
	if accountID > 0 {
		query += fmt.Sprintf(" AND account_id = $%d", len(args)+1)
		args = append(args, accountID)
	}
	if groupID > 0 {
		query += fmt.Sprintf(" AND group_id = $%d", len(args)+1)
		args = append(args, groupID)
	}
	query, args = appendUsageLogModelQueryFilter(query, args, model, modelSource)
	query, args = appendRequestTypeOrStreamQueryFilter(query, args, requestType, stream)
	if billingType != nil {
		query += fmt.Sprintf(" AND billing_type = $%d", len(args)+1)
		args = append(args, int16(*billingType))
	}
	query, args = appendUsageLogBillingModeQueryFilter(query, args, billingMode, "")
	query += " GROUP BY endpoint ORDER BY requests DESC"

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]EndpointStat, 0)
	for rows.Next() {
		var row EndpointStat
		if err := rows.Scan(&row.Endpoint, &row.Requests, &row.TotalTokens, &row.Cost, &row.ActualCost); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetEndpointStatsWithFilters returns inbound endpoint statistics with optional filters.
func (r *usageLogRepository) GetEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]EndpointStat, error) {
	return r.getEndpointStatsByColumnWithFilters(ctx, "inbound_endpoint", startTime, endTime, userID, apiKeyID, accountID, groupID, model, "", requestType, stream, billingType, "")
}

// GetUpstreamEndpointStatsWithFilters returns upstream endpoint statistics with optional filters.
func (r *usageLogRepository) GetUpstreamEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]EndpointStat, error) {
	return r.getEndpointStatsByColumnWithFilters(ctx, "upstream_endpoint", startTime, endTime, userID, apiKeyID, accountID, groupID, model, "", requestType, stream, billingType, "")
}

// GetAccountUsageStats returns comprehensive usage statistics for an account over a time range
func (r *usageLogRepository) GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (resp *AccountUsageStatsResponse, err error) {
	daysCount := int(endTime.Sub(startTime).Hours()/24) + 1
	if daysCount <= 0 {
		daysCount = 30
	}

	query := `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM-DD') as date,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as cost,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as actual_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY date
		ORDER BY date ASC
	`

	rows, err := r.sql.QueryContext(ctx, query, accountID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() {
		// 保持主错误优先；仅在无错误时回传 Close 失败。
		// 同时清空返回值，避免误用不完整结果。
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			resp = nil
		}
	}()

	history := make([]AccountUsageHistory, 0)
	for rows.Next() {
		var date string
		var requests int64
		var tokens int64
		var cost float64
		var actualCost float64
		var userCost float64
		if err = rows.Scan(&date, &requests, &tokens, &cost, &actualCost, &userCost); err != nil {
			return nil, err
		}
		t, _ := time.Parse("2006-01-02", date)
		history = append(history, AccountUsageHistory{
			Date:       date,
			Label:      t.Format("01/02"),
			Requests:   requests,
			Tokens:     tokens,
			Cost:       cost,
			ActualCost: actualCost,
			UserCost:   userCost,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	var totalAccountCost, totalUserCost, totalStandardCost float64
	var totalRequests, totalTokens int64
	var highestCostDay, highestRequestDay *AccountUsageHistory

	for i := range history {
		h := &history[i]
		totalAccountCost += h.ActualCost
		totalUserCost += h.UserCost
		totalStandardCost += h.Cost
		totalRequests += h.Requests
		totalTokens += h.Tokens

		if highestCostDay == nil || h.ActualCost > highestCostDay.ActualCost {
			highestCostDay = h
		}
		if highestRequestDay == nil || h.Requests > highestRequestDay.Requests {
			highestRequestDay = h
		}
	}

	actualDaysUsed := len(history)
	if actualDaysUsed == 0 {
		actualDaysUsed = 1
	}

	avgQuery := "SELECT COALESCE(AVG(duration_ms), 0) as avg_duration_ms FROM usage_logs WHERE account_id = $1 AND created_at >= $2 AND created_at < $3"
	var avgDuration float64
	if err := scanSingleRow(ctx, r.sql, avgQuery, []any{accountID, startTime, endTime}, &avgDuration); err != nil {
		return nil, err
	}

	summary := AccountUsageSummary{
		Days:              daysCount,
		ActualDaysUsed:    actualDaysUsed,
		TotalCost:         totalAccountCost,
		TotalUserCost:     totalUserCost,
		TotalStandardCost: totalStandardCost,
		TotalRequests:     totalRequests,
		TotalTokens:       totalTokens,
		AvgDailyCost:      totalAccountCost / float64(actualDaysUsed),
		AvgDailyUserCost:  totalUserCost / float64(actualDaysUsed),
		AvgDailyRequests:  float64(totalRequests) / float64(actualDaysUsed),
		AvgDailyTokens:    float64(totalTokens) / float64(actualDaysUsed),
		AvgDurationMs:     avgDuration,
	}

	todayStr := timezone.Now().Format("2006-01-02")
	for i := range history {
		if history[i].Date == todayStr {
			summary.Today = &struct {
				Date     string  `json:"date"`
				Cost     float64 `json:"cost"`
				UserCost float64 `json:"user_cost"`
				Requests int64   `json:"requests"`
				Tokens   int64   `json:"tokens"`
			}{
				Date:     history[i].Date,
				Cost:     history[i].ActualCost,
				UserCost: history[i].UserCost,
				Requests: history[i].Requests,
				Tokens:   history[i].Tokens,
			}
			break
		}
	}

	if highestCostDay != nil {
		summary.HighestCostDay = &struct {
			Date     string  `json:"date"`
			Label    string  `json:"label"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
			Requests int64   `json:"requests"`
		}{
			Date:     highestCostDay.Date,
			Label:    highestCostDay.Label,
			Cost:     highestCostDay.ActualCost,
			UserCost: highestCostDay.UserCost,
			Requests: highestCostDay.Requests,
		}
	}

	if highestRequestDay != nil {
		summary.HighestRequestDay = &struct {
			Date     string  `json:"date"`
			Label    string  `json:"label"`
			Requests int64   `json:"requests"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
		}{
			Date:     highestRequestDay.Date,
			Label:    highestRequestDay.Label,
			Requests: highestRequestDay.Requests,
			Cost:     highestRequestDay.ActualCost,
			UserCost: highestRequestDay.UserCost,
		}
	}

	models, err := r.GetModelStatsWithFilters(ctx, startTime, endTime, 0, 0, accountID, 0, nil, nil, nil)
	if err != nil {
		models = []ModelStat{}
	}
	endpoints, endpointErr := r.GetEndpointStatsWithFilters(ctx, startTime, endTime, 0, 0, accountID, 0, "", nil, nil, nil)
	if endpointErr != nil {
		logger.LegacyPrintf("repository.usage_log", "GetEndpointStatsWithFilters failed in GetAccountUsageStats: %v", endpointErr)
		endpoints = []EndpointStat{}
	}
	upstreamEndpoints, upstreamEndpointErr := r.GetUpstreamEndpointStatsWithFilters(ctx, startTime, endTime, 0, 0, accountID, 0, "", nil, nil, nil)
	if upstreamEndpointErr != nil {
		logger.LegacyPrintf("repository.usage_log", "GetUpstreamEndpointStatsWithFilters failed in GetAccountUsageStats: %v", upstreamEndpointErr)
		upstreamEndpoints = []EndpointStat{}
	}

	resp = &AccountUsageStatsResponse{
		History:           history,
		Summary:           summary,
		Models:            models,
		Endpoints:         endpoints,
		UpstreamEndpoints: upstreamEndpoints,
	}
	return resp, nil
}
