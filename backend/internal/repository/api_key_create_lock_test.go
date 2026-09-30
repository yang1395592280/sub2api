package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCreateLock_CheckAndInsertShareLockedTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAPIKeyRepositoryWithSQL(client, db)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT COUNT.*FROM "api_keys"`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(199)))
	mock.ExpectQuery(`INSERT INTO "api_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(200)))
	mock.ExpectCommit()
	err = repo.WithUserCreateLock(context.Background(), 7, func(ctx context.Context) error {
		require.NotNil(t, dbent.TxFromContext(ctx))
		count, err := repo.CountByUserID(ctx, 7)
		require.NoError(t, err)
		require.Equal(t, int64(199), count)
		return repo.Create(ctx, &service.APIKey{UserID: 7, Key: "sk-locked-create", Name: "locked", Status: service.StatusActive})
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCreateLock_RollsBackWhenLimitRejects(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAPIKeyRepositoryWithSQL(client, db)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectRollback()
	err = repo.WithUserCreateLock(context.Background(), 7, func(context.Context) error {
		return service.ErrAPIKeyCountExceeded
	})
	require.ErrorIs(t, err, service.ErrAPIKeyCountExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCreateLock_RespectsOuterTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAPIKeyRepositoryWithSQL(client, db)
	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	callbackErr := errors.New("outer callback failed")
	err = repo.WithUserCreateLock(dbent.NewTxContext(context.Background(), tx), 7, func(ctx context.Context) error {
		require.Same(t, tx, dbent.TxFromContext(ctx))
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	// 回调结束不提交或回滚调用方事务。
	require.NoError(t, mock.ExpectationsWereMet())
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
