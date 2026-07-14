package repository

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerOverviewRepositoryUsesBoundedPostgresAggregates(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := newOpenAISchedulerOverviewRepositoryWithSQL(db)
	start := time.Date(2026, 7, 14, 6, 0, 0, 0, time.UTC)
	end := start.Add(6 * time.Hour)

	mock.ExpectQuery(`(?s)WITH physical_probes AS .*SELECT DISTINCT e\.account_id, e\.model, e\.event_type, e\.created_at.*pg\.deleted_at IS NULL.*PERCENTILE_CONT\(0\.5\).*e2e_first_token_ms.*PERCENTILE_CONT\(0\.95\).*routing_ms.*g\.deleted_at IS NULL`).
		WithArgs(start, end, int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{"e2e_p50", "e2e_p90", "selection_p95", "real_count", "probe_count"}).AddRow(2970.0, 7210.0, 18.0, 100, 20))
	mock.ExpectQuery(`(?s)SELECT g\.id, g\.name, g\.openai_auto_scheduler_enabled,.*account_count,.*e2e_p90.*g\.deleted_at IS NULL`).
		WithArgs(start, end, service.PlatformOpenAI, int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "enabled", "account_count", "e2e_p90"}).AddRow(33, "Codex", true, 4, 7210.0))
	mock.ExpectQuery(`(?s)DATE_TRUNC\('hour', ul\.created_at\).*PERCENTILE_CONT\(0\.5\).*e2e_first_token_ms.*g\.deleted_at IS NULL`).
		WithArgs(start, end, int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{"bucket", "e2e_p50", "e2e_p90"}).AddRow(start, 2500.0, 6100.0))
	mock.ExpectQuery(`(?s)GREATEST\(.*e2e_first_token_ms.*routing_ms.*queue_ms.*retry_ms.*g\.deleted_at IS NULL.*CASE.*upstream_ttft`).
		WithArgs(start, end, 10000, int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{"reason", "count"}).AddRow("queue", 3).AddRow("upstream_ttft", 1))

	got, err := repo.GetOpenAISchedulerOverviewMetrics(context.Background(), service.OpenAISchedulerOverviewParams{
		GroupID: 33, Window: 6 * time.Hour, Bucket: time.Hour, StartTime: start, EndTime: end, SlowThresholdMS: 10000,
	})

	require.NoError(t, err)
	require.Equal(t, 2970.0, got.E2EP50MS)
	require.Equal(t, 18.0, got.SelectionP95MS)
	require.InDelta(t, float64(20)/120, got.ProbeRatio, 0.0001)
	require.Equal(t, "Codex", got.Groups[0].Name)
	require.Equal(t, start, got.Trend[0].Bucket)
	require.Equal(t, int64(3), got.SlowCauses[0].Count)
	require.InDelta(t, 0.75, got.SlowCauses[0].Ratio, 0.0001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildOpenAISchedulerOverviewTrendQueryUsesSixHourBucketsForSevenDays(t *testing.T) {
	query, args := buildOpenAISchedulerOverviewTrendQuery(service.OpenAISchedulerOverviewParams{
		GroupID: 82, Bucket: 6 * time.Hour,
		StartTime: time.Unix(100, 0), EndTime: time.Unix(200, 0),
	})
	require.Contains(t, query, "INTERVAL '6 hours'")
	require.Contains(t, query, "ul.created_at AT TIME ZONE 'UTC'")
	require.Contains(t, query, "AT TIME ZONE 'UTC' AS bucket")
	require.Contains(t, query, "g.deleted_at IS NULL")
	require.NotContains(t, strings.ToUpper(query), "SELECT *")
	require.Equal(t, []any{time.Unix(100, 0), time.Unix(200, 0), int64(82)}, args)
}

func TestBuildOpenAISchedulerOverviewSummaryQueryKeepsDistinctPhysicalAttemptsWithinOneSecond(t *testing.T) {
	query, args := buildOpenAISchedulerOverviewSummaryQuery(service.OpenAISchedulerOverviewParams{
		StartTime: time.Unix(100, 0), EndTime: time.Unix(200, 0),
	})
	require.Contains(t, query, "SELECT DISTINCT e.account_id, e.model, e.event_type, e.created_at")
	require.NotContains(t, query, "DATE_TRUNC('second', e.created_at)")
	require.Contains(t, query, "JOIN groups pg ON pg.id = e.group_id")
	require.Contains(t, query, "pg.deleted_at IS NULL")
	require.Contains(t, query, "g.deleted_at IS NULL")
	require.Equal(t, []any{time.Unix(100, 0), time.Unix(200, 0)}, args)
}

func TestOpenAISchedulerOverviewRepositoryHealthUsesFixedCountAndRowsQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := newOpenAISchedulerOverviewRepositoryWithSQL(db)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	tempBlockedUntil := now.Add(time.Minute)
	accountExpiresAt := now.Add(2 * time.Minute)
	overloadUntil := now.Add(3 * time.Minute)
	rateLimitResetAt := now.Add(4 * time.Minute)
	price := 0.75
	params := service.OpenAISchedulerHealthParams{
		GroupID: 33, State: "running", ModelFamily: "gpt-5.4", Endpoint: "responses", Transport: "http_sse",
		Sort: "predicted_ttft_ms", Order: "asc", Page: 2, PageSize: 20,
	}
	filterArgs := []driver.Value{service.PlatformOpenAI, int64(33), "running", "gpt-5.4", "responses", "http_sse"}

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM account_groups ag.*LEFT JOIN openai_scheduler_health_states hs`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rowArgs := append(append([]driver.Value(nil), filterArgs...), 20, 20)
	mock.ExpectQuery(`(?s)WITH health_rows AS.*g\.status AS group_status.*g\.openai_auto_scheduler_enabled AS group_auto_scheduler_enabled.*a\.status AS account_status.*a\.schedulable.*a\.temp_unschedulable_until.*a\.auto_pause_on_expired.*a\.expires_at AS account_expires_at.*a\.overload_until.*a\.rate_limit_reset_at.*CASE WHEN a\.load_factor > 0 THEN a\.load_factor WHEN a\.concurrency > 0 THEN a\.concurrency ELSE 1 END AS load_capacity.*SELECT account_id, account_name, group_id, group_status, group_auto_scheduler_enabled`).
		WithArgs(rowArgs...).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "account_name", "group_id", "group_status", "group_auto_scheduler_enabled",
			"account_status", "schedulable", "temp_unschedulable_until", "temp_unschedulable_reason",
			"auto_pause_on_expired", "account_expires_at", "overload_until", "rate_limit_reset_at",
			"model_family", "endpoint", "transport", "state", "predicted_ttft_ms", "real_sample_count", "probe_sample_count", "error_rate",
			"rate_limited_rate", "server_error_rate", "cooldown_until", "expires_at", "updated_at", "load_capacity", "channel_price",
		}).AddRow(10, "primary", 33, service.StatusActive, true, service.StatusActive, true, tempBlockedUntil, "price guard", true, accountExpiresAt, overloadUntil, rateLimitResetAt, "gpt-5.4", "responses", "http_sse", "running", 1200.0, 20, 2, 0.01, 0.02, 0.03, nil, now.Add(time.Minute), now, 20, price))

	items, total, err := repo.ListOpenAISchedulerHealth(context.Background(), params)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, int64(10), items[0].AccountID)
	require.Equal(t, "gpt-5.4", items[0].ModelFamily)
	require.Equal(t, 20, items[0].LoadCapacity)
	require.Equal(t, service.StatusActive, items[0].AccountStatus)
	require.Equal(t, service.StatusActive, items[0].GroupStatus)
	require.True(t, items[0].GroupAutoSchedulerEnabled)
	require.True(t, items[0].AutoPauseOnExpired)
	require.Equal(t, tempBlockedUntil, *items[0].TempUnschedulableUntil)
	require.Equal(t, accountExpiresAt, *items[0].AccountExpiresAt)
	require.Equal(t, overloadUntil, *items[0].OverloadUntil)
	require.Equal(t, rateLimitResetAt, *items[0].RateLimitResetAt)
	require.Equal(t, &price, items[0].ChannelPrice)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildOpenAISchedulerHealthQueriesAreExplicitParameterizedAndSortWhitelisted(t *testing.T) {
	countQuery, rowsQuery, countArgs, rowsArgs := buildOpenAISchedulerHealthQueries(service.OpenAISchedulerHealthParams{
		GroupID: 33, State: "running' OR TRUE --", Sort: "drop table", Order: "sideways", Page: 1, PageSize: 5000,
	})
	upper := strings.ToUpper(countQuery + rowsQuery)
	require.NotContains(t, upper, "SELECT *")
	require.NotContains(t, countQuery+rowsQuery, "running' OR TRUE")
	require.NotContains(t, strings.ToLower(rowsQuery), "drop table")
	require.Regexp(t, regexp.MustCompile(`ORDER BY predicted_ttft_ms DESC`), rowsQuery)
	require.Contains(t, countArgs, "running' OR TRUE --")
	require.Equal(t, 200, rowsArgs[len(rowsArgs)-2])
	require.Equal(t, 0, rowsArgs[len(rowsArgs)-1])
}

func TestBuildOpenAISchedulerHealthQueriesSortsSnapshotAgeByInverseTimestamp(t *testing.T) {
	_, rowsQuery, _, _ := buildOpenAISchedulerHealthQueries(service.OpenAISchedulerHealthParams{
		Sort: "snapshot_age_ms", Order: "desc", Page: 1, PageSize: 20,
	})
	require.Contains(t, rowsQuery, "ORDER BY updated_at ASC")
}
