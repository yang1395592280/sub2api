package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBusinessAnalyticsRepository_GetOverviewReadsAggregateTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		GroupID:   7,
	}

	mock.ExpectQuery(containsAllRegexp(
		"FROM business_usage_daily",
		"bucket_date >= $1::date",
		"bucket_date < $2::date",
		"group_id = $3",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(3, 2, 1, 99, 10.0, 6.0, 4.0, 0))

	got, err := repo.GetOverview(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(3), got.Requests)
	require.InDelta(t, 4, got.GrossProfit, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetRecordsUsesHistoricalCostSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessRecordsFilter{
		BusinessAnalyticsFilter: service.BusinessAnalyticsFilter{
			StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		Page:     1,
		PageSize: 20,
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM usage_logs ul LEFT JOIN groups g ON g.id = ul.group_id LEFT JOIN accounts a ON a.id = ul.account_id WHERE ul.created_at >= $1 AND ul.created_at < $2")).
		WithArgs(filter.StartDate, filter.EndDate).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(containsAllRegexp(
		"COALESCE(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1), 0) AS channel_cost",
		"COALESCE(ul.actual_cost, 0) - COALESCE(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1), 0) AS gross_profit",
		"FROM usage_logs ul",
		"ORDER BY ul.created_at DESC, ul.id DESC",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.PageSize, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "user_id", "user_email", "api_key_id", "api_key_name", "group_id", "group_name", "account_id", "account_name", "model", "requests", "total_tokens", "revenue", "channel_cost", "gross_profit"}).
			AddRow(1, filter.StartDate, 2, "u@example.com", 3, "key", 4, "group", 5, "account", "model", 1, 10, 10.0, 6.0, 4.0))

	got, err := repo.GetRecords(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(1), got.Total)
	require.Len(t, got.Items, 1)
	require.InDelta(t, 4, got.Items[0].GrossProfit, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}
