package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPelicanCreateBatchSkipsAccountsAlreadyRunning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	createdAt := time.Now()
	columns := []string{"id", "batch_id", "account_id", "model_id", "prompt", "status", "response_text", "html", "error_message", "latency_ms", "created_at", "started_at", "finished_at"}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO account_pelican_tests").
		WithArgs("batch", int64(11), "gpt-6-astra", "pelican").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(int64(1), "batch", int64(11), "gpt-6-astra", "pelican", "queued", "", "", "", int64(0), createdAt, nil, nil))
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
