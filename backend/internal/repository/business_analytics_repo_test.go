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

func TestBusinessAnalyticsRepository_GetOverviewWeeklyHistoricalAggregatesFromDailyBuckets(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		Granularity: "week",
		GroupID:     7,
	}

	mock.ExpectQuery(containsAllRegexp(
		"FROM business_usage_daily b",
		"b.bucket_date >= $1::date",
		"b.bucket_date < $2::date",
		"b.group_id = $3",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(30, 8, 4, 990, 100.0, 60.0, 40.0, 1))

	got, err := repo.GetOverview(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(30), got.Requests)
	require.Equal(t, int64(8), got.ActiveUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetTrendWeeklyHistoricalAggregatesDailyBucketsAndDistinctUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	filter := service.BusinessAnalyticsFilter{
		StartDate:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		Granularity: "week",
		GroupID:     7,
	}

	mock.ExpectQuery(containsAllRegexp(
		"WITH usage_totals AS",
		"date_trunc('week', b.bucket_date)::date::text date",
		"FROM business_usage_daily b",
		"b.bucket_date >= $1::date",
		"b.bucket_date < $2::date",
		"GROUP BY date_trunc('week', b.bucket_date)::date",
		"active_users AS",
		"COUNT(DISTINCT bu.user_id)",
		"date_trunc('week', bu.bucket_date)::date",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.GroupID).
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "active_users", "revenue", "channel_cost", "gross_profit"}).
			AddRow("2026-06-01", 20, 5, 80.0, 50.0, 30.0))

	got, err := repo.GetTrend(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(5), got[0].ActiveUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetTrendHistoricalCountsDistinctUsers(t *testing.T) {
	query, _ := buildBusinessAggregateQuery(service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	}, "bucket_date", true)

	require.Contains(t, query, "FROM business_usage_daily_users bu")
	require.Contains(t, query, "COUNT(DISTINCT bu.user_id)")
	require.NotContains(t, query, "SUM(active_users)")
}

func TestBusinessAnalyticsRepository_GetTrendIncludingTodayWeeklyUsesWeeklyBuckets(t *testing.T) {
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	query, _ := buildBusinessAggregateQuery(service.BusinessAnalyticsFilter{
		StartDate:   todayStart.AddDate(0, 0, -7),
		EndDate:     todayStart.AddDate(0, 0, 1),
		Granularity: "week",
	}, "bucket_date", true)

	require.Contains(t, query, "date_trunc('week', ul.created_at)::date::text")
	require.Contains(t, query, "GROUP BY date_trunc('week', ul.created_at)::date")
	require.NotContains(t, query, "ul.created_at::date::text")
	require.NotContains(t, query, "GROUP BY ul.created_at::date")
}

func TestBusinessAnalyticsRepository_GetOverviewIncludingTodayUsesChannelPriceSnapshotCost(t *testing.T) {
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	query, _ := buildBusinessAggregateQuery(service.BusinessAnalyticsFilter{
		StartDate: todayStart,
		EndDate:   todayStart.AddDate(0, 0, 1),
	}, "", false)

	require.Contains(t, query, "COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1))), 0)")
	require.NotContains(t, query, "COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)")
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
		"SUM(b.avg_group_rate_multiplier * GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL)",
		"NULLIF(SUM(GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL), 0)",
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
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "platform", "rate", "avg_rate", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "previous_revenue", "previous_gross_profit"}).
			AddRow(7, "group", "openai", 1.0, 1.125, 10, 3, 2, 99, 10.0, 6.0, 4.0, 1.0, 0.5))

	got, err := repo.GetGroups(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ActiveUsers)
	require.Equal(t, int64(2), got[0].ActiveAPIKeys)
	require.NotNil(t, got[0].AverageRateMultiplier)
	require.InDelta(t, 1.125, *got[0].AverageRateMultiplier, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetGroupsHistoricalUsesWeightedAverageRateMultiplier(t *testing.T) {
	filter := service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	}
	previous := service.BusinessAnalyticsFilter{
		StartDate: filter.StartDate.AddDate(0, 0, -3),
		EndDate:   filter.StartDate,
	}

	query, _ := buildBusinessGroupsQuery(filter, previous)

	require.Contains(t, query, "SUM(b.avg_group_rate_multiplier * GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL)")
	require.Contains(t, query, "NULLIF(SUM(GREATEST(b.revenue, 0.000000001)) FILTER (WHERE b.avg_group_rate_multiplier IS NOT NULL), 0)")
	require.NotContains(t, query, "AVG(b.avg_group_rate_multiplier)")
	require.NotContains(t, query, "AVG(avg_group_rate_multiplier)")
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
		"COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1))), 0) AS channel_cost",
		"COALESCE(SUM(ul.actual_cost), 0) - COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1))), 0) AS gross_profit",
		"SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL)",
		"NULLIF(SUM(GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL), 0) AS avg_group_rate_multiplier",
		"FROM usage_logs ul",
		"LEFT JOIN groups g ON g.id = ul.group_id",
		"LEFT JOIN accounts a ON a.id = ul.account_id",
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
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "group_name", "platform", "rate", "avg_rate", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "previous_revenue", "previous_gross_profit"}).
			AddRow(7, "group", "openai", 1.0, 1.125, 10, 3, 2, 99, 10.0, 6.0, 4.0, 1.0, 0.5))

	got, err := repo.GetGroups(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetGroupsIncludingTodayPlatformFilterJoinsPlatformTables(t *testing.T) {
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	filter := service.BusinessAnalyticsFilter{
		StartDate: todayStart,
		EndDate:   todayStart.AddDate(0, 0, 1),
		Platform:  "openai",
	}
	previous := service.BusinessAnalyticsFilter{
		StartDate: filter.StartDate.AddDate(0, 0, -1),
		EndDate:   filter.StartDate,
		Platform:  filter.Platform,
	}

	query, args := buildBusinessGroupsQuery(filter, previous)

	require.Contains(t, query, "FROM usage_logs ul")
	require.Contains(t, query, "LEFT JOIN groups g ON g.id = ul.group_id")
	require.Contains(t, query, "LEFT JOIN accounts a ON a.id = ul.account_id")
	require.Contains(t, query, "COALESCE("+usageLogEffectivePlatformExpr+", '') = $3")
	require.Contains(t, query, "SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL)")
	require.Contains(t, query, "NULLIF(SUM(GREATEST(ul.actual_cost, 0.000000001)) FILTER (WHERE ul.rate_multiplier IS NOT NULL), 0)")
	require.NotContains(t, query, "AVG(ul.rate_multiplier)")
	require.Equal(t, []any{filter.StartDate, filter.EndDate, filter.Platform, previous.StartDate, previous.EndDate, previous.Platform}, args)
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
		"SUM(b.avg_channel_price * GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL)",
		"NULLIF(SUM(GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL), 0)",
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
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "account_name", "channel_id", "platform", "status", "channel_price", "balance_status", "avg_channel_price", "requests", "active_users", "active_api_keys", "total_tokens", "revenue", "channel_cost", "gross_profit", "missing"}).
			AddRow(11, "account", 3, "openai", "normal", 1.0, "", 0.875, 10, 3, 2, 99, 10.0, 6.0, 4.0, 0))

	got, err := repo.GetChannels(context.Background(), filter)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ActiveUsers)
	require.Equal(t, int64(2), got[0].ActiveAPIKeys)
	require.NotNil(t, got[0].AverageChannelPrice)
	require.InDelta(t, 0.875, *got[0].AverageChannelPrice, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBusinessAnalyticsRepository_GetChannelsHistoricalUsesSnapshotWeightedAveragePrice(t *testing.T) {
	query, _ := buildBusinessChannelsQuery(service.BusinessAnalyticsFilter{
		StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	})

	require.Contains(t, query, "SUM(b.avg_channel_price * GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL)")
	require.Contains(t, query, "NULLIF(SUM(GREATEST(b.requests - b.missing_channel_price_records, 0)) FILTER (WHERE b.avg_channel_price IS NOT NULL), 0)")
	require.NotContains(t, query, "AVG(b.avg_channel_price)")
}

func TestBusinessAnalyticsRepository_GetChannelsIncludingTodayUsesSnapshotCountWeightedAveragePrice(t *testing.T) {
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	query, _ := buildBusinessChannelsQuery(service.BusinessAnalyticsFilter{
		StartDate: todayStart,
		EndDate:   todayStart.AddDate(0, 0, 1),
	})

	require.Contains(t, query, "SUM(ul.channel_price_snapshot) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL)")
	require.Contains(t, query, "NULLIF(COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL), 0)")
	require.Contains(t, query, "COUNT(*) FILTER (WHERE ul.channel_price_snapshot IS NOT NULL)")
	require.Contains(t, query, "COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1))), 0)")
	require.NotContains(t, query, "AVG(ul.channel_price_snapshot)")
	require.NotContains(t, query, "COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)")
}

func TestBusinessAnalyticsRepository_GetPriceChangeImpactReturnsExpandedMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newBusinessAnalyticsRepositoryWithSQL(db)
	changeDate := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	input := service.PriceChangeImpactInput{
		GroupID:    7,
		ChangeDate: changeDate,
		Days:       7,
	}

	query, _ := buildPriceChangeImpactQuery(input)
	require.Contains(t, query, "before_period AS")
	require.Contains(t, query, "after_period AS")
	require.Contains(t, query, "before_users AS")
	require.Contains(t, query, "after_users AS")
	require.Contains(t, query, "AS before_requests")
	require.Contains(t, query, "AS after_requests")
	require.Contains(t, query, "AS before_active_users")
	require.Contains(t, query, "AS after_active_users")
	require.Contains(t, query, "AS before_channel_cost")
	require.Contains(t, query, "AS after_channel_cost")
	require.Contains(t, query, "AS before_avg_rate_multiplier")
	require.Contains(t, query, "AS after_avg_rate_multiplier")
	require.Contains(t, query, "AS new_users")
	require.Contains(t, query, "AS lost_users")

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(changeDate.AddDate(0, 0, -7), changeDate, changeDate, changeDate.AddDate(0, 0, 7), input.GroupID).
		WillReturnRows(sqlmock.NewRows([]string{
			"before_requests", "after_requests",
			"before_active_users", "after_active_users",
			"before_revenue", "after_revenue", "revenue_delta",
			"before_channel_cost", "after_channel_cost",
			"before_gross_profit", "after_gross_profit", "gross_profit_delta",
			"before_avg_rate_multiplier", "after_avg_rate_multiplier",
			"new_users", "lost_users",
		}).AddRow(
			10, 12,
			4, 5,
			8.0, 12.0, 4.0,
			5.0, 6.0,
			3.0, 6.0, 3.0,
			1.2, 1.4,
			2, 1,
		))

	got, err := repo.GetPriceChangeImpact(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(10), got.BeforeRequests)
	require.Equal(t, int64(12), got.AfterRequests)
	require.Equal(t, int64(2), got.NewUsers)
	require.Equal(t, int64(1), got.LostUsers)
	require.NotNil(t, got.BeforeAvgRateMultiplier)
	require.InDelta(t, 1.2, *got.BeforeAvgRateMultiplier, 0.000001)
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
		"COALESCE(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1)), 0) AS channel_cost",
		"COALESCE(ul.actual_cost, 0) - COALESCE(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.channel_price_snapshot, COALESCE(ul.account_rate_multiplier, 1)), 0) AS gross_profit",
		"ul.rate_multiplier",
		"ul.channel_price_snapshot",
		"ul.channel_price_snapshot IS NULL AS channel_price_snapshot_missing",
		"FROM usage_logs ul",
		"ORDER BY ul.created_at DESC, ul.id DESC",
	)).
		WithArgs(filter.StartDate, filter.EndDate, filter.PageSize, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "user_id", "user_email", "api_key_id", "api_key_name", "group_id", "group_name", "account_id", "account_name", "model", "requests", "total_tokens", "revenue", "channel_cost", "gross_profit", "rate_multiplier", "channel_price_snapshot", "channel_price_snapshot_missing"}).
			AddRow(1, filter.StartDate, 2, "u@example.com", 3, "key", 4, "group", 5, "account", "model", 1, 10, 10.0, 6.0, 4.0, 1.125, 0.75, false))

	got, err := repo.GetRecords(context.Background(), filter)

	require.NoError(t, err)
	require.Equal(t, int64(1), got.Total)
	require.Len(t, got.Items, 1)
	require.InDelta(t, 4, got.Items[0].GrossProfit, 0.000001)
	require.NotNil(t, got.Items[0].RateMultiplier)
	require.InDelta(t, 1.125, *got.Items[0].RateMultiplier, 0.000001)
	require.NotNil(t, got.Items[0].ChannelPriceSnapshot)
	require.InDelta(t, 0.75, *got.Items[0].ChannelPriceSnapshot, 0.000001)
	require.False(t, got.Items[0].ChannelPriceSnapshotMissing)
	require.NoError(t, mock.ExpectationsWereMet())
}
