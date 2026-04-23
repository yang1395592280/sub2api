package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLeaderboardRefreshSQLExcludesRefundedRoundsFromStats(t *testing.T) {
	require.Contains(t, leaderboardRefreshSQL, "status IN ('won', 'lost')")
	require.NotContains(t, leaderboardRefreshSQL, "'refunded'")
}

func TestSizeBetRepositoryListLeaderboardFiltersZeroBetCount(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &sizeBetRepository{db: db}
	updatedAt := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"user_id", "username", "net_profit", "win_count", "bet_count", "updated_at",
	}).AddRow(int64(9), "alice", 12.5, int64(2), int64(3), updatedAt)

	mock.ExpectQuery("FROM game_rank_snapshots grs.*grs\\.bet_count > 0").
		WithArgs("all", "all", 20).
		WillReturnRows(rows)

	items, _, err := repo.ListLeaderboard(context.Background(), "all", "all", 20)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
