package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func newOpenAISchedulerStatsSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestOpenAISchedulerStatsRepository_IncrementDailySelection(t *testing.T) {
	db, mock := newOpenAISchedulerStatsSQLMock(t)
	repo := NewOpenAISchedulerStatsRepository(db)
	statDate := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	selectedAt := time.Date(2026, 6, 13, 10, 30, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO openai_scheduler_daily_stats").
		WithArgs(statDate, int64(33), int64(11855), selectedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.IncrementDailySelection(context.Background(), statDate, 33, 11855, selectedAt)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAISchedulerStatsRepository_GetDailyStatsCalculatesRatios(t *testing.T) {
	db, mock := newOpenAISchedulerStatsSQLMock(t)
	repo := NewOpenAISchedulerStatsRepository(db)
	statDate := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	lastSelected := time.Date(2026, 6, 13, 11, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"account_id", "select_count", "last_selected_at"}).
		AddRow(int64(11855), int64(80), lastSelected).
		AddRow(int64(11845), int64(20), lastSelected.Add(-time.Hour))
	mock.ExpectQuery("SELECT account_id, select_count, last_selected_at").
		WithArgs(statDate, int64(33)).
		WillReturnRows(rows)

	stats, err := repo.GetDailyStats(context.Background(), statDate, 33)

	require.NoError(t, err)
	require.Equal(t, "2026-06-13", stats.Date)
	require.Equal(t, int64(33), stats.GroupID)
	require.Equal(t, int64(100), stats.TotalSelects)
	require.Len(t, stats.Accounts, 2)
	require.Equal(t, int64(11855), stats.Accounts[0].AccountID)
	require.Equal(t, 0.8, stats.Accounts[0].SelectRatio)
	require.Equal(t, int64(11845), stats.Accounts[1].AccountID)
	require.Equal(t, 0.2, stats.Accounts[1].SelectRatio)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAISchedulerStatsRepository_RecomputeDailyStatsFromUsageLogs(t *testing.T) {
	db, mock := newOpenAISchedulerStatsSQLMock(t)
	repo := NewOpenAISchedulerStatsRepository(db)
	statDate := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	start := statDate
	end := statDate.Add(24 * time.Hour)
	lastSelected := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM openai_scheduler_daily_stats").
		WithArgs(statDate, int64(33)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO openai_scheduler_daily_stats").
		WithArgs(statDate, int64(33), start, end).
		WillReturnResult(sqlmock.NewResult(0, 2))
	rows := sqlmock.NewRows([]string{"account_id", "select_count", "last_selected_at"}).
		AddRow(int64(11855), int64(2), lastSelected)
	mock.ExpectQuery("SELECT account_id, select_count, last_selected_at").
		WithArgs(statDate, int64(33)).
		WillReturnRows(rows)
	mock.ExpectCommit()

	stats, err := repo.RecomputeDailyStatsFromUsageLogs(context.Background(), statDate, start, end, 33)

	require.NoError(t, err)
	require.Equal(t, int64(2), stats.TotalSelects)
	require.Len(t, stats.Accounts, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
