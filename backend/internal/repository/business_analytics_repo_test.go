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

func TestBusinessAnalyticsRepository_GetOverviewHistoricalCountsDistinctUsersAndAPIKeysAcrossRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		GroupID:   7,
		AccountID: 11,
		Platform:  "openai",
	}

	mock.ExpectQuery(containsAllRegexp(
		"WITH usage_totals AS",
		"FROM business_usage_daily b",
		"b.bucket_date < $2::date",
		"b.group_id = $3",
		"b.account_id = $4",
		"b.platform = $5",
		"active_users AS",
		"COUNT(DISTINCT bu.user_id)",
		"FROM business_usage_daily_users bu",
		"JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id",
		"b.bucket_date < $2::date",
		"b.group_id = $3",
		"b.account_id = $4",
		"b.platform = $5",
		"active_api_keys AS",
		"COUNT(DISTINCT ul.api_key_id)",
		"FROM usage_logs ul",
		"COALESCE("+usageLogEffectivePlatformExpr+", '') = $5",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID, filter.AccountID, filter.Platform).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(10, 3, 2, 99, 10.0, 6.0, 4.0, 0))

	got, err := repo.GetOverview(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(3), got.ActiveUsers)
	require.Equal(t, int64(2), got.ActiveAPIKeys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetOverviewEndDateAtTodayStartReadsAggregateTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: todayStart.AddDate(0, 0, -1),
		EndDate:   todayStart,
	}

	mock.ExpectQuery(containsAllRegexp(
		"FROM business_usage_daily",
		"bucket_date >= $1::date",
		"bucket_date < $2::date",
	)).
		WithArgs(filter.StartDate, filter.EndDate).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(2, 1, 1, 42, 8.0, 5.0, 3.0, 0))

	got, err := repo.GetOverview(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(2), got.Requests)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetGroupsHistoricalCountsDistinctUsersAcrossRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		AccountID: 11,
		Platform:  "openai",
	}

	mock.ExpectQuery(containsAllRegexp(
		"WITH current_usage AS",
		"SUM(requests) requests",
		"b.bucket_date < $2::date",
		"b.account_id = $3",
		"b.platform = $4",
		"active_users AS",
		"SELECT bu.group_id, COUNT(DISTINCT bu.user_id) active_users",
		"JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id",
		"b.bucket_date < $2::date",
		"b.account_id = $3",
		"b.platform = $4",
		"active_api_keys AS",
		"SELECT COALESCE(ul.group_id, 0) AS group_id, COUNT(DISTINCT ul.api_key_id) active_api_keys",
		"COALESCE("+usageLogEffectivePlatformExpr+", '') = $4",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.AccountID, filter.Platform, filter.StartDate.AddDate(0, 0, -3), filter.StartDate, filter.AccountID, filter.Platform).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "platform", "rate", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "previous_revenue", "previous_gross_profit"}).
			AddRow(7, "group", "openai", 1.0, 10, 3, 2, 99, 10.0, 6.0, 4.0, 1.0, 0.5))

	got, err := repo.GetGroups(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ActiveUsers)
	require.Equal(t, int64(2), got[0].ActiveAPIKeys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetGroupsIncludingTodayAliasesPreviousPeriod(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: todayStart,
		EndDate:   todayStart.AddDate(0, 0, 1),
		GroupID:   7,
		AccountID: 11,
		Platform:  "openai",
	}

	mock.ExpectQuery(containsAllRegexp(
		"WITH current_period AS",
		"COALESCE(ul.group_id, 0) AS group_id",
		"FROM usage_logs ul",
		"ul.created_at >= $1",
		"ul.created_at < $2",
		"previous_period AS",
		"FROM business_usage_daily p",
		"p.bucket_date >= $6::date",
		"p.bucket_date < $7::date",
		"p.group_id = $8",
		"p.account_id = $9",
		"p.platform = $10",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID, filter.AccountID, filter.Platform, filter.StartDate.AddDate(0, 0, -1), filter.StartDate, filter.GroupID, filter.AccountID, filter.Platform).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "platform", "rate", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "previous_revenue", "previous_gross_profit"}).
			AddRow(7, "group", "openai", 1.0, 10, 3, 2, 99, 10.0, 6.0, 4.0, 1.0, 0.5))

	got, err := repo.GetGroups(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetChannelsHistoricalQualifiesDailyFiltersInActiveUserCTE(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		GroupID:   7,
		AccountID: 11,
		Platform:  "openai",
	}

	mock.ExpectQuery(containsAllRegexp(
		"WITH account_usage AS",
		"FROM business_usage_daily b",
		"b.bucket_date < $2::date",
		"b.group_id = $3",
		"b.account_id = $4",
		"b.platform = $5",
		"active_users AS",
		"SELECT bu.account_id, COUNT(DISTINCT bu.user_id) active_users",
		"JOIN business_usage_daily b ON b.bucket_date = bu.bucket_date AND b.group_id = bu.group_id AND b.account_id = bu.account_id",
		"b.bucket_date < $2::date",
		"b.group_id = $3",
		"b.account_id = $4",
		"b.platform = $5",
		"active_api_keys AS",
		"COALESCE("+usageLogEffectivePlatformExpr+", '') = $5",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID, filter.AccountID, filter.Platform).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "account_name", "channel_id", "platform", "status", "channel_price", "balance_status", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(11, "account", 3, "openai", "normal", 1.0, "", 10, 3, 2, 99, 10.0, 6.0, 4.0, 0))

	got, err := repo.GetChannels(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ActiveUsers)
	require.Equal(t, int64(2), got[0].ActiveAPIKeys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_HistoricalQueriesDoNotUseBareDailyFilterColumns(t *testing.T) {
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		GroupID:   7,
		AccountID: 11,
		Platform:  "openai",
	}
	previous := service.BusinessAnalyticsFilter{
		StartDate: filter.StartDate.AddDate(0, 0, -3),
		EndDate:   filter.StartDate,
		GroupID:   filter.GroupID,
		AccountID: filter.AccountID,
		Platform:  filter.Platform,
	}

	overviewSQL, _ := buildBusinessAggregateQuery(filter, "", false)
	groupsSQL, _ := buildBusinessGroupsQuery(filter, previous)
	channelsSQL, _ := buildBusinessChannelsQuery(filter)

	for name, query := range map[string]string{
		"overview": overviewSQL,
		"groups":   groupsSQL,
		"channels": channelsSQL,
	} {
		t.Run(name, func(t *testing.T) {
			require.NotContains(t, query, " group_id =")
			require.NotContains(t, query, " account_id =")
			require.NotContains(t, query, " platform =")
			require.Contains(t, query, "b.group_id =")
			require.Contains(t, query, "b.account_id =")
			require.Contains(t, query, "b.platform =")
		})
	}
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
