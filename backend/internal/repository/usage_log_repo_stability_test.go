package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetAccountStabilityStatsBatch(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"account_id", "success_count", "error_count", "avg_duration_ms"}).
		AddRow(int64(7), int64(99), int64(1), int64(1200)).
		AddRow(int64(8), int64(0), int64(3), nil)

	mock.ExpectQuery("WITH account_ids AS").
		WithArgs(sqlmock.AnyArg(), start, end).
		WillReturnRows(rows)

	stats, err := repo.GetAccountStabilityStatsBatch(context.Background(), []int64{7, 8, 9}, start, end)

	require.NoError(t, err)
	require.Equal(t, int64(99), stats[7].SuccessCount)
	require.Equal(t, int64(1), stats[7].ErrorCount)
	require.NotNil(t, stats[7].AvgDurationMs)
	require.Equal(t, 1200, *stats[7].AvgDurationMs)
	require.Equal(t, int64(0), stats[8].SuccessCount)
	require.Equal(t, int64(3), stats[8].ErrorCount)
	require.Nil(t, stats[8].AvgDurationMs)
	require.Equal(t, int64(0), stats[9].SuccessCount)
	require.Equal(t, int64(0), stats[9].ErrorCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
