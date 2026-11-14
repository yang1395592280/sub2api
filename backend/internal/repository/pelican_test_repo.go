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
	total_tokens, created_at, started_at, finished_at, source_group_id, source_group_name, test_kind, answer, estimated_cost_usd, upstream_rate_multiplier, cost_model_id, question_id, question_name, expected_answer`

func scanPelicanTest(row interface{ Scan(...any) error }) (service.PelicanTest, error) {
	var test service.PelicanTest
	err := row.Scan(&test.ID, &test.BatchID, &test.AccountID, &test.ModelID, &test.Prompt,
		&test.Status, &test.ResponseText, &test.HTML, &test.ErrorMessage, &test.LatencyMS,
		&test.InputTokens, &test.OutputTokens, &test.TotalTokens,
		&test.CreatedAt, &test.StartedAt, &test.FinishedAt, &test.SourceGroupID, &test.SourceGroupName, &test.TestKind, &test.Answer, &test.EstimatedCostUSD, &test.UpstreamRateMultiplier, &test.CostModelID, &test.QuestionID, &test.QuestionName, &test.ExpectedAnswer)
	return test, err
}

func (r *pelicanTestRepository) ExpireStale(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `UPDATE account_pelican_tests
		SET status = 'failed', error_message = '后台任务已中断或超时', finished_at = NOW()
		WHERE (status = 'running' AND started_at < NOW() - INTERVAL '10 minutes')
		   OR (status = 'queued' AND created_at < NOW() - INTERVAL '6 hours')`)
	return err
}

func (r *pelicanTestRepository) CreateBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, prompt string, kinds ...string) ([]service.PelicanTest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	tests := make([]service.PelicanTest, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		row := tx.QueryRowContext(ctx, `INSERT INTO account_pelican_tests
			(batch_id, account_id, model_id, prompt, test_kind, question_id, question_name, expected_answer)
			VALUES ($1, $2, $3, $4, $5::text,
			 COALESCE((SELECT id FROM generation_test_questions WHERE test_kind=$5::text AND prompt=$4 ORDER BY id LIMIT 1),0),
			 COALESCE((SELECT name FROM generation_test_questions WHERE test_kind=$5::text AND prompt=$4 ORDER BY id LIMIT 1),''),
			 CASE WHEN $5::text='intelligence' THEN '20' ELSE '' END)
			ON CONFLICT (account_id, test_kind, question_id) WHERE status IN ('queued', 'running') DO NOTHING
			RETURNING `+pelicanTestColumns, batchID, accountID, modelID, prompt, service.GenerationTestKind(kinds...))
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

func (r *pelicanTestRepository) Finish(ctx context.Context, id int64, status, responseText, html, errorMessage string, latencyMS int64, usage service.AccountTestTokenUsage, details ...service.PelicanTest) error {
	var summary service.PelicanTest
	if len(details) > 0 {
		summary = details[0]
	}
	_, err := r.db.ExecContext(ctx, `UPDATE account_pelican_tests
		SET status = $2, response_text = $3, html = $4, error_message = $5,
			latency_ms = $6, input_tokens = $7, output_tokens = $8,
			total_tokens = $9, answer=$10, estimated_cost_usd=$11, upstream_rate_multiplier=$12, cost_model_id=$13, finished_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'running')`,
		id, status, responseText, html, errorMessage, latencyMS,
		usage.InputTokens, usage.OutputTokens, usage.TotalTokens, summary.Answer, summary.EstimatedCostUSD, summary.UpstreamRateMultiplier, summary.CostModelID)
	return err
}

func (r *pelicanTestRepository) ListLatest(ctx context.Context, accountIDs []int64, kinds ...string) ([]service.PelicanTest, error) {
	if len(accountIDs) == 0 {
		return []service.PelicanTest{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT ON (account_id) `+pelicanSummaryColumns+`
        FROM account_pelican_tests WHERE account_id = ANY($1) AND test_kind=$2
        ORDER BY account_id, id DESC`, pq.Array(accountIDs), service.GenerationTestKind(kinds...))
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

func (r *pelicanTestRepository) Get(ctx context.Context, id int64, kinds ...string) (*service.PelicanTest, error) {
	test, err := scanPelicanTest(r.db.QueryRowContext(ctx, `SELECT `+pelicanTestColumns+`
		FROM account_pelican_tests WHERE id = $1 AND test_kind=$2`, id, service.GenerationTestKind(kinds...)))
	if err != nil {
		return nil, err
	}
	return &test, nil
}
