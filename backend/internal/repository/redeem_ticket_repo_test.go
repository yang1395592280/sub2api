package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newRedeemTicketRepoMock(t *testing.T) (*redeemCodeRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &redeemCodeRepository{client: client}, mock
}

func expectRedeemTicketWalletReconcile(mock sqlmock.Sqlmock, userID int64, balance int) {
	mock.ExpectExec(`INSERT INTO zenxiang_liyu_ticket_wallets`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT balance FROM zenxiang_liyu_ticket_wallets`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
	mock.ExpectExec(`UPDATE zenxiang_liyu_ticket_batches`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`UPDATE zenxiang_liyu_ticket_wallets`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
}

func TestGrantRedeemTicketsAddsOneExpiringBatch(t *testing.T) {
	repo, mock := newRedeemTicketRepoMock(t)
	expectRedeemTicketWalletReconcile(mock, 42, 2)
	mock.ExpectExec(`(?s)INSERT INTO zenxiang_liyu_ticket_batches.*date \+ \$4::integer`).
		WithArgs(int64(42), "TICKET-CODE", 2, service.ZenxiangLiyuTicketRetentionDays).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE zenxiang_liyu_ticket_wallets`).
		WithArgs(2, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.GrantRedeemTickets(context.Background(), 42, "TICKET-CODE", 2))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantRedeemTicketsRejectsCapacityWithoutConsumingCodeBenefit(t *testing.T) {
	repo, mock := newRedeemTicketRepoMock(t)
	expectRedeemTicketWalletReconcile(mock, 42, 4)

	err := repo.GrantRedeemTickets(context.Background(), 42, "TICKET-CODE", 2)

	require.ErrorIs(t, err, service.ErrRedeemTicketCapacity)
	require.NoError(t, mock.ExpectationsWereMet())
}
