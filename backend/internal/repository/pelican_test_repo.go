package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type pelicanTestRepository struct{ db *sql.DB }

func NewPelicanTestRepository(db *sql.DB) service.PelicanTestRepository {
	return &pelicanTestRepository{db: db}
}

const pelicanTestColumns = `id, batch_id, account_id, model_id, prompt, status,
	response_text, html, error_message, latency_ms, input_tokens, output_tokens,
	total_tokens, created_at, started_at, finished_at`

func scanPelicanTest(row interface{ Scan(...any) error }) (service.PelicanTest, error) {
	var test service.PelicanTest
	err := row.Scan(&test.ID, &test.BatchID, &test.AccountID, &test.ModelID, &test.Prompt,
		&test.Status, &test.ResponseText, &test.HTML, &test.ErrorMessage, &test.LatencyMS,
		&test.InputTokens, &test.OutputTokens, &test.TotalTokens,
		&test.CreatedAt, &test.StartedAt, &test.FinishedAt)
	return test, err
}

func (r *pelicanTestRepository) ExpireStale(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `UPDATE account_pelican_tests
		SET status = 'failed', error_message = '后台任务已中断或超时', finished_at = NOW()
		WHERE (status = 'running' AND started_at < NOW() - INTERVAL '10 minutes')
		   OR (status = 'queued' AND created_at < NOW() - INTERVAL '6 hours')`)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM account_pelican_tests
		WHERE created_at < NOW() - INTERVAL '30 days'`)
	return err
}

func (r *pelicanTestRepository) CreateBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, prompt string) ([]service.PelicanTest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	tests := make([]service.PelicanTest, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		row := tx.QueryRowContext(ctx, `INSERT INTO account_pelican_tests
			(batch_id, account_id, model_id, prompt)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (account_id) WHERE status IN ('queued', 'running') DO NOTHING
			RETURNING `+pelicanTestColumns, batchID, accountID, modelID, prompt)
		test, scanErr := scanPelicanTest(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			continue
		}
		if scanErr != nil {
			return nil, scanErr
		}
		tests = append(tests, test)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tests, nil
}

func (r *pelicanTestRepository) MarkRunning(ctx context.Context, id int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE account_pelican_tests
		SET status = 'running', started_at = NOW()
		WHERE id = $1 AND status = 'queued'`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *pelicanTestRepository) Finish(ctx context.Context, id int64, status, responseText, html, errorMessage string, latencyMS int64, usage service.AccountTestTokenUsage) error {
	_, err := r.db.ExecContext(ctx, `UPDATE account_pelican_tests
		SET status = $2, response_text = $3, html = $4, error_message = $5,
			latency_ms = $6, input_tokens = $7, output_tokens = $8,
			total_tokens = $9, finished_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'running')`,
		id, status, responseText, html, errorMessage, latencyMS,
		usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
	if err != nil {
		return err
	}
	// Keep a small recent history per account to bound generated HTML storage.
	_, err = r.db.ExecContext(ctx, `DELETE FROM account_pelican_tests
		WHERE account_id = (SELECT account_id FROM account_pelican_tests WHERE id = $1)
		AND id NOT IN (
			SELECT id FROM account_pelican_tests
			WHERE account_id = (SELECT account_id FROM account_pelican_tests WHERE id = $1)
			ORDER BY id DESC LIMIT 3
		)`, id)
	return err
}

func (r *pelicanTestRepository) ListLatest(ctx context.Context, accountIDs []int64) ([]service.PelicanTest, error) {
	if len(accountIDs) == 0 {
		return []service.PelicanTest{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT ON (account_id)
		id, batch_id, account_id, model_id, '' AS prompt, status,
		'' AS response_text, '' AS html, error_message, latency_ms,
		input_tokens, output_tokens, total_tokens,
		created_at, started_at, finished_at
		FROM account_pelican_tests WHERE account_id = ANY($1)
		ORDER BY account_id, id DESC`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tests := make([]service.PelicanTest, 0, len(accountIDs))
	for rows.Next() {
		test, err := scanPelicanTest(rows)
		if err != nil {
			return nil, err
		}
		tests = append(tests, test)
	}
	return tests, rows.Err()
}

func (r *pelicanTestRepository) Get(ctx context.Context, id int64) (*service.PelicanTest, error) {
	test, err := scanPelicanTest(r.db.QueryRowContext(ctx, `SELECT `+pelicanTestColumns+`
		FROM account_pelican_tests WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return &test, nil
}
