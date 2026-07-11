package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestZenxiangLiyuRepositoryPlayReturnsExistingRecordForSameRequestID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &zenxiangLiyuRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "user_id", "ticket_amount", "reward_amount", "user_net_amount",
			"system_revenue", "system_expense", "system_profit", "prize_id", "prize_name_snapshot",
			"probability_snapshot", "balance_before", "balance_after_ticket", "balance_after_reward", "created_at", "lucky_coin_played",
		}).AddRow(9, "req-1", 42, 2.0, 3.0, 1.0, 2.0, 3.0, -1.0, 7, "3 yuan", 20.0, 12.0, 10.0, 13.0, time.Unix(1, 0), false))
	mock.ExpectCommit()

	result, err := repo.Play(context.Background(), service.ZenxiangLiyuPlayCommand{UserID: 42, RequestID: "req-1"})
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.InDelta(t, 3, result.RewardAmount, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestZenxiangLiyuRepositoryPlayAppliesAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	playDate := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	usageStart, usageEnd := zenxiangLiyuUsageWindow(playDate)
	repo := &zenxiangLiyuRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,`).
		WillReturnRows(zenxiangLiyuSettingsRows().AddRow(true, 0.0, 10.0, 5, 5.0, 3, 0.1, 0.05, true, 50.0))
	mock.ExpectQuery(`SELECT id, name, reward_amount, probability, enabled, sort_order FROM zenxiang_liyu_prizes WHERE enabled = TRUE ORDER BY sort_order, id FOR SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_amount", "probability", "enabled", "sort_order"}).AddRow(7, "3 yuan", 3.0, 100.0, true, 1))
	mock.ExpectQuery(`SELECT id, email, role, status, balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "status", "balance"}).AddRow(42, "user@example.com", service.RoleUser, service.StatusActive, 12.0))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(actual_cost\), 0\)`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(5.01))
	mock.ExpectQuery(`SELECT EXISTS\(`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(false))
	mock.ExpectQuery(`SELECT GREATEST\(`).
		WithArgs(int64(42), playDate, usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`UPDATE users SET balance = balance - \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`).
		WithArgs(0.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(12.0))
	mock.ExpectQuery(`INSERT INTO zenxiang_liyu_records`).
		WithArgs(
			"req-2", int64(42), playDate, 0.0, 3.0, 3.0, 0.0, 3.0, -3.0, int64(7), "3 yuan", 100.0,
			sqlmock.AnyArg(), 12.0, 12.0, 15.0,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(9, time.Unix(2, 0)))
	mock.ExpectQuery(`UPDATE users SET balance = balance \+ \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`).
		WithArgs(3.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(15.0))
	mock.ExpectCommit()

	result, err := repo.Play(context.Background(), service.ZenxiangLiyuPlayCommand{
		UserID:    42,
		RequestID: "req-2",
		PlayDate:  playDate,
		Roll:      50,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, "req-2", result.RequestID)
	require.InDelta(t, 15, result.BalanceAfterReward, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestZenxiangLiyuRepositoryPlayUsesFreePlayAfterDailyUsageThreshold(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	playDate := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	usageStart, usageEnd := zenxiangLiyuUsageWindow(playDate)
	repo := &zenxiangLiyuRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-free").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,`).
		WillReturnRows(zenxiangLiyuSettingsRows().AddRow(true, 0.0, 10.0, 5, 5.0, 3, 0.1, 0.05, true, 50.0))
	mock.ExpectQuery(`SELECT id, name, reward_amount, probability, enabled, sort_order FROM zenxiang_liyu_prizes WHERE enabled = TRUE ORDER BY sort_order, id FOR SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_amount", "probability", "enabled", "sort_order"}).AddRow(7, "3 yuan", 3.0, 100.0, true, 1))
	mock.ExpectQuery(`SELECT id, email, role, status, balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "status", "balance"}).AddRow(42, "user@example.com", service.RoleUser, service.StatusActive, 1.0))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(actual_cost\), 0\)`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(5.01))
	mock.ExpectQuery(`SELECT EXISTS\(`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(false))
	mock.ExpectQuery(`SELECT GREATEST\(`).
		WithArgs(int64(42), playDate, usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`UPDATE users SET balance = balance - \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`).
		WithArgs(0.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(1.0))
	mock.ExpectQuery(`INSERT INTO zenxiang_liyu_records`).
		WithArgs(
			"req-free", int64(42), playDate, 0.0, 3.0, 3.0, 0.0, 3.0, -3.0, int64(7), "3 yuan", 100.0,
			sqlmock.AnyArg(), 1.0, 1.0, 4.0,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(9, time.Unix(4, 0)))
	mock.ExpectQuery(`UPDATE users SET balance = balance \+ \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`).
		WithArgs(3.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(4.0))
	mock.ExpectCommit()

	result, err := repo.Play(context.Background(), service.ZenxiangLiyuPlayCommand{
		UserID:    42,
		RequestID: "req-free",
		PlayDate:  playDate,
		Roll:      50,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.FreePlay)
	require.Zero(t, result.TicketAmount)
	require.InDelta(t, 4, result.BalanceAfterReward, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestZenxiangLiyuRepositoryPlayReturnsExistingRecordAfterUniqueConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	playDate := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	usageStart, usageEnd := zenxiangLiyuUsageWindow(playDate)
	repo := &zenxiangLiyuRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-conflict").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,`).
		WillReturnRows(zenxiangLiyuSettingsRows().AddRow(true, 0.0, 10.0, 5, 5.0, 3, 0.1, 0.05, true, 50.0))
	mock.ExpectQuery(`SELECT id, name, reward_amount, probability, enabled, sort_order FROM zenxiang_liyu_prizes WHERE enabled = TRUE ORDER BY sort_order, id FOR SHARE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_amount", "probability", "enabled", "sort_order"}).AddRow(7, "3 yuan", 3.0, 100.0, true, 1))
	mock.ExpectQuery(`SELECT id, email, role, status, balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "role", "status", "balance"}).AddRow(42, "user@example.com", service.RoleUser, service.StatusActive, 12.0))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(actual_cost\), 0\)`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(5.01))
	mock.ExpectQuery(`SELECT EXISTS\(`).
		WithArgs(int64(42), usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(false))
	mock.ExpectQuery(`SELECT GREATEST\(`).
		WithArgs(int64(42), playDate, usageStart, usageEnd).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`UPDATE users SET balance = balance - \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`).
		WithArgs(0.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(12.0))
	mock.ExpectQuery(`INSERT INTO zenxiang_liyu_records`).
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-conflict").
		WillReturnRows(zenxiangLiyuRecordRows().AddRow(9, "req-conflict", 42, 2.0, 3.0, 1.0, 2.0, 3.0, -1.0, 7, "3 yuan", 20.0, 12.0, 10.0, 13.0, time.Unix(3, 0), false))
	mock.ExpectCommit()

	result, err := repo.Play(context.Background(), service.ZenxiangLiyuPlayCommand{
		UserID:    42,
		RequestID: "req-conflict",
		PlayDate:  playDate,
		Roll:      50,
	})
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, "req-conflict", result.RequestID)
	require.InDelta(t, 3, result.RewardAmount, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestZenxiangLiyuRepositorySavePrizesDisablesOmittedPrizes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM zenxiang_liyu_settings WHERE id = 1 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO zenxiang_liyu_prizes`).
		WithArgs("1 yuan", 1.0, 100.0, true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_amount", "probability", "enabled", "sort_order"}).
			AddRow(8, "1 yuan", 1.0, 100.0, true, 1))
	mock.ExpectQuery(`UPDATE zenxiang_liyu_prizes`).
		WithArgs("2 yuan", 2.0, 0.0, false, 2, int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_amount", "probability", "enabled", "sort_order"}).
			AddRow(3, "2 yuan", 2.0, 0.0, false, 2))
	mock.ExpectExec(`UPDATE zenxiang_liyu_prizes SET enabled = FALSE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &zenxiangLiyuRepository{db: db}
	prizes, err := repo.SavePrizes(context.Background(), []service.ZenxiangLiyuPrize{
		{Name: "1 yuan", RewardAmount: 1, Probability: 100, Enabled: true, SortOrder: 1},
		{ID: 3, Name: "2 yuan", RewardAmount: 2, Probability: 0, Enabled: false, SortOrder: 2},
	})
	require.NoError(t, err)
	require.Len(t, prizes, 2)
	require.Equal(t, int64(8), prizes[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestZenxiangLiyuRepositoryPlayChecksGrantInsideTransactionWhenGlobalDisabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	playDate := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	repo := &zenxiangLiyuRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(42), "req-grant").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,`).
		WillReturnRows(zenxiangLiyuSettingsRows().AddRow(false, 0.0, 10.0, 5, 5.0, 3, 0.1, 0.05, true, 50.0))
	mock.ExpectQuery(`SELECT enabled FROM zenxiang_liyu_user_grants WHERE user_id = \$1 FOR SHARE`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
	mock.ExpectRollback()

	_, err = repo.Play(context.Background(), service.ZenxiangLiyuPlayCommand{UserID: 42, RequestID: "req-grant", PlayDate: playDate, Roll: 50})
	require.ErrorIs(t, err, service.ErrZenxiangLiyuUnauthorized)
	require.NoError(t, mock.ExpectationsWereMet())
}

func zenxiangLiyuRecordRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "request_id", "user_id", "ticket_amount", "reward_amount", "user_net_amount",
		"system_revenue", "system_expense", "system_profit", "prize_id", "prize_name_snapshot",
		"probability_snapshot", "balance_before", "balance_after_ticket", "balance_after_reward", "created_at", "lucky_coin_played",
	})
}

func zenxiangLiyuSettingsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"global_enabled", "ticket_amount", "minimum_balance", "daily_play_limit",
		"ticket_usage_threshold", "daily_ticket_limit", "unit_sale_price", "unit_cost_price",
		"lucky_coin_enabled", "lucky_coin_double_probability",
	})
}
