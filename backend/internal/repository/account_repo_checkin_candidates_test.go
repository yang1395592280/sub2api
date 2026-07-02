package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListSub2APICheckinCandidates(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var capturedSQL string
	mock.ExpectQuery("SELECT id").
		WillReturnRows(sqlmock.NewRows([]string{"id"})).
		WillDelayFor(0)

	repo := &accountRepository{sql: captureQuerySQL{db: db, captured: &capturedSQL}}

	accounts, err := repo.ListSub2APICheckinCandidates(context.Background(), 10)
	require.NoError(t, err)
	require.Empty(t, accounts)

	normalized := normalizeSQLWhitespace(capturedSQL)
	require.Contains(t, normalized, "deleted_at IS NULL")
	require.Contains(t, normalized, "status = 'active'")
	require.Contains(t, normalized, "type = 'apikey'")
	require.Contains(t, normalized, "platform IN ('openai', 'anthropic')")
	require.Contains(t, normalized, `credentials @> '{"upstream_admin_type":"sub2api"}'::jsonb`)
	require.Contains(t, normalized, `credentials @> '{"upstream_checkin_enabled":true}'::jsonb`)
	require.Contains(t, normalized, "ORDER BY priority ASC, id ASC")
	require.Contains(t, normalized, "LIMIT $1")
	require.NoError(t, mock.ExpectationsWereMet())
}
