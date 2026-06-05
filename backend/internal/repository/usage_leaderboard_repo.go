package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageLeaderboardRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUsageLeaderboardRepository(client *dbent.Client, sqlDB *sql.DB) service.UsageLeaderboardRepository {
	return &usageLeaderboardRepository{
		client: client,
		sql:    sqlDB,
	}
}

func (r *usageLeaderboardRepository) ListUsageLeaderboard(ctx context.Context, date time.Time, metric service.UsageLeaderboardMetric, params pagination.PaginationParams) (items []service.UsageLeaderboardRawItem, result *pagination.PaginationResult, err error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	total, err := r.CountUsageLeaderboardParticipants(ctx, date, metric)
	if err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.UsageLeaderboardRawItem{}, &pagination.PaginationResult{
			Total:    0,
			Page:     params.Page,
			PageSize: params.PageSize,
			Pages:    0,
		}, nil
	}

	offset := (params.Page - 1) * params.PageSize
	orderExpr := usageLeaderboardMetricExpr(metric)
	query := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				ul.user_id,
				COALESCE(MAX(NULLIF(u.username, '')), '') AS username,
				COALESCE(MAX(NULLIF(u.email, '')), '') AS email,
				COUNT(*) FILTER (WHERE %s) AS requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_read_tokens + ul.cache_creation_tokens), 0) AS tokens
			FROM usage_logs ul
			LEFT JOIN users u ON u.id = ul.user_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.user_id > 0
			GROUP BY ul.user_id
			HAVING %s > 0
		)
		SELECT
			ROW_NUMBER() OVER (ORDER BY %s DESC, user_id ASC) AS rank,
			user_id,
			username,
			email,
			requests,
			tokens
		FROM ranked
		ORDER BY %s DESC, user_id ASC
		LIMIT $3 OFFSET $4
	`, usageLeaderboardSuccessExpr(), orderExpr, orderExpr, orderExpr)

	rows, err := r.sql.QueryContext(ctx, query, date, date.AddDate(0, 0, 1), params.PageSize, offset)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			items = nil
			result = nil
		}
	}()

	items = make([]service.UsageLeaderboardRawItem, 0)
	for rows.Next() {
		var item service.UsageLeaderboardRawItem
		if err := rows.Scan(&item.Rank, &item.UserID, &item.Username, &item.Email, &item.Requests, &item.Tokens); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    int((total + int64(params.PageSize) - 1) / int64(params.PageSize)),
	}, nil
}

func (r *usageLeaderboardRepository) GetUsageLeaderboardCurrentUserEntry(ctx context.Context, date time.Time, metric service.UsageLeaderboardMetric, userID int64) (*service.UsageLeaderboardRawItem, error) {
	orderExpr := usageLeaderboardMetricExpr(metric)
	query := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				ul.user_id,
				COALESCE(MAX(NULLIF(u.username, '')), '') AS username,
				COALESCE(MAX(NULLIF(u.email, '')), '') AS email,
				COUNT(*) FILTER (WHERE %s) AS requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_read_tokens + ul.cache_creation_tokens), 0) AS tokens
			FROM usage_logs ul
			LEFT JOIN users u ON u.id = ul.user_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.user_id > 0
			GROUP BY ul.user_id
			HAVING %s > 0
		)
		SELECT rank, user_id, username, email, requests, tokens
		FROM (
			SELECT
				ROW_NUMBER() OVER (ORDER BY %s DESC, user_id ASC) AS rank,
				user_id,
				username,
				email,
				requests,
				tokens
			FROM ranked
		) ranked_rows
		WHERE user_id = $3
	`, usageLeaderboardSuccessExpr(), orderExpr, orderExpr)

	var item service.UsageLeaderboardRawItem
	err := scanSingleRow(ctx, r.sql, query, []any{date, date.AddDate(0, 0, 1), userID}, &item.Rank, &item.UserID, &item.Username, &item.Email, &item.Requests, &item.Tokens)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *usageLeaderboardRepository) CountUsageLeaderboardParticipants(ctx context.Context, date time.Time, metric service.UsageLeaderboardMetric) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT ul.user_id
			FROM usage_logs ul
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND ul.user_id > 0
			GROUP BY ul.user_id
			HAVING %s > 0
		) AS ranked_users
	`, usageLeaderboardMetricExpr(metric))

	var total int64
	if err := scanSingleRow(ctx, r.sql, query, []any{date, date.AddDate(0, 0, 1)}, &total); err != nil {
		return 0, err
	}
	return total, nil
}

func usageLeaderboardSuccessExpr() string {
	return "ul.actual_cost > 0"
}

func usageLeaderboardMetricExpr(metric service.UsageLeaderboardMetric) string {
	switch metric {
	case service.UsageLeaderboardMetricTokens:
		return "COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_read_tokens + ul.cache_creation_tokens), 0)"
	default:
		return fmt.Sprintf("COUNT(*) FILTER (WHERE %s)", usageLeaderboardSuccessExpr())
	}
}
