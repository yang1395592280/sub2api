package accountquality

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestRunsFiltersUseSnapshotAndPaginateTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	clause := "TRUE AND policy_id=$1 AND account_id=$2 AND (snapshot->>'source_group_id')::bigint=$3 AND (snapshot->'condition'->>'model_id'=$4 OR snapshot->>'effective_upstream_model'=$4) AND strpos(lower(COALESCE(snapshot->>'account_name','')),lower($5))>0 AND verdict=$6 AND COALESCE(started_at,queued_at)>=$7 AND COALESCE(started_at,queued_at)<$8"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(id) FROM account_quality_test_runs WHERE "+clause)).WithArgs(int64(1), int64(42), int64(7), "gpt-5.5", "upstream'_%", "passed", from, to).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(51))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+strings.Replace(runColumns, ",response_text,", ",NULL AS response_text,", 1)+" FROM account_quality_test_runs WHERE "+clause+" ORDER BY COALESCE(started_at,queued_at) DESC,id DESC LIMIT $9 OFFSET $10")).WithArgs(int64(1), int64(42), int64(7), "gpt-5.5", "upstream'_%", "passed", from, to, 25, 25).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	q := &Tx{sql: tx, ctx: context.Background()}
	items, total, err := q.Runs(RunFilter{PolicyID: 1, AccountID: 42, SourceGroupID: 7, ModelID: "gpt-5.5", Search: "upstream'_%", Verdict: "passed", From: &from, To: &to, Limit: 25, Offset: 25})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, 51, total)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEventsPaginationKeepsEventOrdering(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	where := " WHERE ($1::bigint=0 OR policy_id=$1) AND ($2::bigint=0 OR account_id=$2) AND ($3::bigint=0 OR run_id=$3) AND ($4='' OR event_type=$4) AND ($5::timestamptz IS NULL OR created_at>=$5) AND ($6::timestamptz IS NULL OR created_at<$6)"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(id) FROM account_quality_events"+where)).WithArgs(int64(1), int64(42), int64(0), "", &from, &to).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(51))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,document,created_at,run_id FROM account_quality_events"+where+" ORDER BY id DESC LIMIT $7 OFFSET $8")).WithArgs(int64(1), int64(42), int64(0), "", &from, &to, 25, 25).WillReturnRows(sqlmock.NewRows([]string{"id", "document", "created_at", "run_id"}).AddRow(int64(7), []byte(`{"event_type":"state_changed"}`), to, int64(3)))
	q := &Tx{sql: tx, ctx: context.Background()}
	items, total, err := q.Events(1, 42, 0, "", &from, &to, 25, 25)
	require.NoError(t, err)
	require.Equal(t, 51, total)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].ID)
	require.Equal(t, to, items[0].CreatedAt)
	require.NotNil(t, items[0].RunID)
	require.Equal(t, int64(3), *items[0].RunID)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
