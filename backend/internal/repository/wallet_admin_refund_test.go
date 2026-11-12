package repository

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

const refundReplaySQL = "SELECT paid_delta,checkin_delta,liyu_delta,gift_delta,notes FROM wallet_transactions WHERE user_id="

func TestWalletHistoryAmountDisplay(t *testing.T) {
	for _, tc := range []struct {
		value float64
		text  string
	}{
		{0, "+0.00"}, {-0.00000001, "+0.00"}, {1.005, "+1.01"}, {-1.005, "-1.01"}, {-0.0414776, "-0.04"},
	} {
		require.Equal(t, tc.text, walletHistoryAmount(tc.value))
	}
}

func refundRepo(t *testing.T) (*userRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	return &userRepository{client: client, sql: db}, mock
}
func expectNewRefund(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(refundReplaySQL).WithArgs(int64(1), "op").WillReturnRows(sqlmock.NewRows([]string{"paid_delta", "checkin_delta", "liyu_delta", "gift_delta", "notes"}))
}
func TestAdminWalletRefundSourceIsolation(t *testing.T) {
	for _, tc := range []struct {
		source               string
		paid, welfare        float64
		before, delta, after service.WalletBalance
	}{
		{"checkin", 1, 2, service.WalletBalance{Paid: 5, Checkin: 3, Liyu: -4, Gift: 8}, service.WalletBalance{Paid: -1, Checkin: -2}, service.WalletBalance{Paid: 4, Checkin: 1, Liyu: -4, Gift: 8}},
		{"liyu", 0, 2, service.WalletBalance{Paid: -1, Checkin: -4, Liyu: 3, Gift: 8}, service.WalletBalance{Liyu: -2}, service.WalletBalance{Paid: -1, Checkin: -4, Liyu: 1, Gift: 8}},
		{"gift", 1, 2, service.WalletBalance{Paid: 5, Checkin: -4, Liyu: 3, Gift: 8}, service.WalletBalance{Paid: -1, Gift: -2}, service.WalletBalance{Paid: 4, Checkin: -4, Liyu: 3, Gift: 6}},
		{"", 5, 0, service.WalletBalance{Paid: 5, Checkin: -4, Liyu: 3, Gift: 8}, service.WalletBalance{Paid: -5}, service.WalletBalance{Checkin: -4, Liyu: 3, Gift: 8}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			repo, mock := refundRepo(t)
			mock.ExpectBegin()
			expectWalletLock(mock, 1, tc.before)
			expectNewRefund(mock)
			expectWalletDelta(mock, 1, tc.delta, tc.after, "admin_refund", "op", 0)
			mock.ExpectCommit()
			require.NoError(t, repo.RefundWallet(context.Background(), 1, tc.paid, tc.welfare, tc.source, 0, "", "op"))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAdminWalletRefundRejectsInsufficientAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, source  string
		paid, welfare float64
	}{
		{"paid insufficient", "gift", 6, 1}, {"negative source", "checkin", 1, .01}, {"other sources cannot substitute", "liyu", 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := refundRepo(t)
			mock.ExpectBegin()
			expectWalletLock(mock, 1, service.WalletBalance{Paid: 5, Checkin: -1, Liyu: 3, Gift: 100})
			expectNewRefund(mock)
			mock.ExpectRollback()
			require.Error(t, repo.RefundWallet(context.Background(), 1, tc.paid, tc.welfare, tc.source, 0, "", "op"))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	for _, tc := range []struct {
		paid, welfare float64
		source        string
	}{{-1, 1, "gift"}, {1, -1, "gift"}, {math.NaN(), 1, "gift"}, {1, math.Inf(1), "gift"}, {0, 0, "gift"}, {0, 1, ""}, {0, 1, "unknown"}} {
		repo, mock := refundRepo(t)
		require.Error(t, repo.RefundWallet(context.Background(), 1, tc.paid, tc.welfare, tc.source, 0, "", "op"))
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
func TestAdminWalletRefundReplayBeforeBalanceCheck(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "replay", true: "conflict"}[conflict], func(t *testing.T) {
			repo, mock := refundRepo(t)
			mock.ExpectBegin()
			expectWalletLock(mock, 1, service.WalletBalance{})
			previousGift := -2.
			if conflict {
				previousGift = -3
			}
			mock.ExpectQuery(refundReplaySQL).WithArgs(int64(1), "op").WillReturnRows(sqlmock.NewRows([]string{"paid_delta", "checkin_delta", "liyu_delta", "gift_delta", "notes"}).AddRow(-5, 0, 0, previousGift, "reason"))
			if conflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			err := repo.RefundWallet(context.Background(), 1, 5, 2, "gift", 9, "reason", "op")
			if conflict {
				require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAdminWalletRefundLedgerFailureRollsBack(t *testing.T) {
	repo, mock := refundRepo(t)
	mock.ExpectBegin()
	expectWalletLock(mock, 1, service.WalletBalance{Paid: 5, Gift: 3})
	expectNewRefund(mock)
	mock.ExpectQuery(walletDeltaSQL).WithArgs(-1., 0., 0., -2., int64(1)).WillReturnRows(walletRows(service.WalletBalance{Paid: 4, Gift: 1}))
	mock.ExpectExec("INSERT INTO wallet_transactions").WithArgs(int64(1), "admin_refund", "op", int64(0), -1., 0., 0., -2., 4., 0., 0., 1., int64(9), "reason").WillReturnError(errors.New("ledger unavailable"))
	mock.ExpectRollback()
	require.ErrorContains(t, repo.RefundWallet(context.Background(), 1, 1, 2, "gift", 9, "reason", "op"), "ledger unavailable")
	require.NoError(t, mock.ExpectationsWereMet())
}
