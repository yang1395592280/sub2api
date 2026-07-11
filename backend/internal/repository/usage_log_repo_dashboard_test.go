package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFillDashboardEntityStatsCountsOpenAIAutoCheapestUsers(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	stats := &usagestats.DashboardStats{}
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)

	mock.ExpectQuery("FROM users\\s+WHERE deleted_at IS NULL").
		WithArgs(today).
		WillReturnRows(sqlmock.NewRows([]string{"total_users", "today_new_users"}).AddRow(int64(10), int64(2)))
	mock.ExpectQuery("group_select_mode = \\$2 AND u.deleted_at IS NULL").
		WithArgs(service.StatusActive, service.APIKeyGroupSelectModeOpenAIAutoCheapest).
		WillReturnRows(sqlmock.NewRows([]string{"total_api_keys", "active_api_keys", "openai_auto_cheapest_users"}).AddRow(int64(17), int64(15), int64(151)))
	mock.ExpectQuery("FROM accounts\\s+WHERE deleted_at IS NULL").
		WithArgs(service.StatusActive, service.StatusError, now, now).
		WillReturnRows(sqlmock.NewRows([]string{"total_accounts", "normal_accounts", "error_accounts", "ratelimit_accounts", "overload_accounts"}).AddRow(int64(5), int64(4), int64(1), int64(0), int64(0)))

	err := repo.fillDashboardEntityStats(context.Background(), stats, today, now)
	require.NoError(t, err)
	require.Equal(t, int64(151), stats.OpenAIAutoCheapestUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}
