package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBusinessAnalyticsAggregationRepository_RecomputeDaily_SQLIntent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsAggregationRepositoryWithSQL(db)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM business_usage_daily WHERE bucket_date >= $1::date AND bucket_date < $2::date")).
		WithArgs(start, end).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM business_usage_daily_users WHERE bucket_date >= $1::date AND bucket_date < $2::date")).
		WithArgs(start, end).
		WillReturnResult(sqlmock.NewResult(0, 7))
	mock.ExpectExec(containsAllRegexp(
		"INSERT INTO business_usage_daily_users",
		"COALESCE(ul.group_id, 0)",
		"COALESCE(ul.account_id, 0)",
		"COALESCE(SUM(ul.actual_cost), 0) AS revenue",
		"COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS channel_cost",
		"COALESCE(SUM(ul.actual_cost), 0) - COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS gross_profit",
		"GROUP BY 1, 2, 3, 4",
	)).
		WithArgs(start, end).
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec(containsAllRegexp(
		"INSERT INTO business_usage_daily",
		"COALESCE(ul.group_id, 0) AS group_id",
		"COALESCE(ul.account_id, 0) AS account_id",
		"COALESCE(MIN(ul.channel_id), 0) AS channel_id",
		"COALESCE(SUM(ul.actual_cost), 0) AS revenue",
		"COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS channel_cost",
		"COALESCE(SUM(ul.actual_cost), 0) - COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS gross_profit",
		"SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) / SUM(GREATEST(ul.actual_cost, 0.000000001))",
		"AVG(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL)",
		"COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL) AS missing_channel_price_records",
		"GROUP BY 1, 2, 3",
	)).
		WithArgs(start, end).
		WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()

	require.NoError(t, repo.RecomputeDaily(context.Background(), start, end))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsAggregationRepository_RecomputeWeekly_SQLIntent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsAggregationRepositoryWithSQL(db)
	weekStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 7)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM business_usage_weekly WHERE week_start = $1::date")).
		WithArgs(weekStart).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(containsAllRegexp(
		"INSERT INTO business_usage_weekly",
		"$1::date AS week_start",
		"COALESCE(ul.group_id, 0) AS group_id",
		"COALESCE(ul.account_id, 0) AS account_id",
		"COALESCE(MIN(ul.channel_id), 0) AS channel_id",
		"COUNT(DISTINCT ul.user_id) AS active_users",
		"COUNT(DISTINCT ul.api_key_id) AS active_api_keys",
		"COALESCE(SUM(ul.actual_cost), 0) AS revenue",
		"COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS channel_cost",
		"COALESCE(SUM(ul.actual_cost), 0) - COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS gross_profit",
		"AVG(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL)",
		"COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NULL) AS missing_channel_price_records",
		"GROUP BY 2, 3",
	)).
		WithArgs(weekStart, weekEnd).
		WillReturnResult(sqlmock.NewResult(0, 6))
	mock.ExpectCommit()

	require.NoError(t, repo.RecomputeWeekly(context.Background(), weekStart))
	require.NoError(t, mock.ExpectationsWereMet())
}

func containsAllRegexp(parts ...string) string {
	var b strings.Builder
	b.WriteString("(?s)")
	for _, part := range parts {
		b.WriteString(".*")
		b.WriteString(regexp.QuoteMeta(part))
	}
	b.WriteString(".*")
	return b.String()
}
