package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPelicanCreateBatchSkipsAccountsAlreadyRunning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	createdAt := time.Now()
	columns := []string{"id", "batch_id", "account_id", "model_id", "prompt", "status", "response_text", "html", "error_message", "latency_ms", "input_tokens", "output_tokens", "total_tokens", "created_at", "started_at", "finished_at", "source_group_id", "source_group_name"}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO account_pelican_tests").
		WithArgs("batch", int64(11), "gpt-6-astra", "pelican").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(int64(1), "batch", int64(11), "gpt-6-astra", "pelican", "queued", "", "", "", int64(0), nil, nil, nil, createdAt, nil, nil, 0, ""))
	mock.ExpectQuery("INSERT INTO account_pelican_tests").
		WithArgs("batch", int64(12), "gpt-6-astra", "pelican").
		WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectCommit()

	repo := NewPelicanTestRepository(db)
	tests, err := repo.CreateBatch(context.Background(), "batch", []int64{11, 12}, "gpt-6-astra", "pelican")
	require.NoError(t, err)
	require.Len(t, tests, 1)
	require.Equal(t, int64(11), tests[0].AccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPelicanMarkRunningSkipsExpiredQueuedJob(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectExec("UPDATE account_pelican_tests").
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewPelicanTestRepository(db)
	started, err := repo.MarkRunning(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, started)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPelicanHistoryIsNotDeletedWhenFinishingOrExpiring(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("UPDATE account_pelican_tests").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE account_pelican_tests").WillReturnResult(sqlmock.NewResult(0, 0))
	repo := NewPelicanTestRepository(db)
	require.NoError(t, repo.Finish(context.Background(), 1, "failed", "", "", "timeout", 500, service.AccountTestTokenUsage{}))
	require.NoError(t, repo.ExpireStale(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPelicanGroupEnqueueAndSlotAdvanceAreAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT s.group_id.*FOR UPDATE OF s SKIP LOCKED").WillReturnRows(sqlmock.NewRows([]string{"group_id", "model_id", "prompt", "name"}).AddRow(7, "gpt-5.5", "pelican", "正常分组"))
			insert := mock.ExpectExec("INSERT INTO account_pelican_tests.*a.status='active' AND a.schedulable.*ON CONFLICT").WithArgs(sqlmock.AnyArg(), "gpt-5.5", "pelican", int64(7), "正常分组")
			if fail {
				insert.WillReturnError(errors.New("insert failed"))
				mock.ExpectRollback()
			} else {
				insert.WillReturnResult(sqlmock.NewResult(0, 120))
				mock.ExpectExec("UPDATE group_pelican_test_schedules").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = NewPelicanTestRepository(db).EnqueueDueGroups(context.Background())
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPelicanHistoryPaginationOmitsLargeArtworkBodies(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM account_pelican_tests WHERE account_id=\$1`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(40))
	mock.ExpectQuery("SELECT .*'' AS response_text, '' AS html.*ORDER BY id DESC LIMIT").WithArgs(int64(7), 12, 12).WillReturnRows(sqlmock.NewRows([]string{"id", "batch_id", "account_id", "model_id", "prompt", "status", "response_text", "html", "error_message", "latency_ms", "input_tokens", "output_tokens", "total_tokens", "created_at", "started_at", "finished_at", "source_group_id", "source_group_name"}))
	history, err := NewPelicanTestRepository(db).ListHistory(context.Background(), 7, 2, 12)
	require.NoError(t, err)
	require.EqualValues(t, 40, history.Total)
	require.Empty(t, history.Items)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPelicanClaimReleasesTransactionWhenQueueIsEmptyOrLockFails(t *testing.T) {
	for _, lockFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("lockFails=%t", lockFails), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			lock := mock.ExpectExec(`SELECT pg_advisory_xact_lock`)
			if lockFails {
				lock.WillReturnError(errors.New("lock unavailable"))
			} else {
				lock.WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery(`UPDATE account_pelican_tests.*COUNT\(\*\).*<3.*FOR UPDATE SKIP LOCKED`).WillReturnError(sql.ErrNoRows)
			}
			mock.ExpectRollback()
			test, err := NewPelicanTestRepository(db).ClaimNext(context.Background())
			if lockFails {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Nil(t, test)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
