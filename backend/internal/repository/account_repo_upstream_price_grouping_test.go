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

func TestMoveAccountBetweenUpstreamPriceGroupsWritesMembershipAndOutboxAtomically(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)WITH existing AS .*DELETE FROM account_groups.*INSERT INTO account_groups.*INSERT INTO scheduler_outbox`).
		WithArgs(int64(42), sqlmock.AnyArg(), int64(20), service.SchedulerOutboxEventAccountGroupsChanged).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	changed, err := repo.MoveAccountBetweenUpstreamPriceGroups(context.Background(), 42, []int64{10, 20}, 20)

	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMoveAccountBetweenUpstreamPriceGroupsRejectsTargetOutsideManagedScope(t *testing.T) {
	repo := &accountRepository{}

	changed, err := repo.MoveAccountBetweenUpstreamPriceGroups(context.Background(), 42, []int64{10, 20}, 30)

	require.ErrorContains(t, err, "outside upstream price grouping scope")
	require.False(t, changed)
}
