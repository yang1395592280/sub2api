package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

func TestSizeBetRepositoryListUserHistoryUsesLatestRelatedLedgerBalance(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &sizeBetRepository{db: db}
	placedAt := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	settledAt := time.Date(2026, 4, 23, 12, 1, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM game_bets gb.*JOIN game_rounds gr ON gr.id = gb.round_id.*WHERE gr.game_key = \\$1 AND gb.user_id = \\$2").
		WithArgs(service.SizeBetGameKey, int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	rows := sqlmock.NewRows([]string{
		"id", "round_id", "round_no", "direction",
		"result_number", "result_direction",
		"stake_amount", "payout_amount", "net_result_amount", "status",
		"points_after", "placed_at", "settled_at",
	}).AddRow(
		int64(7), int64(11), int64(1002), "big",
		int64(9), "big",
		10.0, 20.0, 10.0, "won",
		int64(123), placedAt, settledAt,
	)

	mock.ExpectQuery("SELECT.*LEFT JOIN LATERAL \\(.*SELECT ROUND\\(balance_after\\)::bigint AS points_after.*FROM game_wallet_ledger gl.*gl.entry_type IN \\('bet_payout', 'bet_refund', 'bet_debit'\\).*ORDER BY gl.created_at DESC, gl.id DESC.*LIMIT 1.*\\) gl ON TRUE.*ORDER BY COALESCE\\(gb.settled_at, gb.placed_at\\) DESC, gb.id DESC").
		WithArgs(service.SizeBetGameKey, int64(9), 20, 0).
		WillReturnRows(rows)

	items, pageResult, err := repo.ListUserHistory(context.Background(), 9, pagination.PaginationParams{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].BetID)
	require.Equal(t, service.SizeBetDirectionBig, items[0].Direction)
	require.Equal(t, service.SizeBetDirectionBig, items[0].ResultDirection)
	require.NotNil(t, items[0].ResultNumber)
	require.Equal(t, 9, *items[0].ResultNumber)
	require.NotNil(t, items[0].PointsAfter)
	require.Equal(t, int64(123), *items[0].PointsAfter)
	require.NotNil(t, items[0].SettledAt)
	require.Equal(t, settledAt, *items[0].SettledAt)
	require.NotNil(t, pageResult)
	require.Equal(t, int64(1), pageResult.Total)
	require.Equal(t, 1, pageResult.Page)
	require.Equal(t, 20, pageResult.PageSize)
	require.Equal(t, 1, pageResult.Pages)
	require.NoError(t, mock.ExpectationsWereMet())
}
