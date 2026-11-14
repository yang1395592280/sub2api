package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const generationQuestionColumns = `id, test_kind, name, prompt, expected_answer, enabled, created_at, updated_at`

func scanGenerationQuestion(row interface{ Scan(...any) error }) (service.GenerationTestQuestion, error) {
	var q service.GenerationTestQuestion
	err := row.Scan(&q.ID, &q.TestKind, &q.Name, &q.Prompt, &q.ExpectedAnswer, &q.Enabled, &q.CreatedAt, &q.UpdatedAt)
	return q, err
}

// Legacy custom schedules can migrate more items than the new-create cap.
// Keep every migrated item readable so administrators can still maintain it.
func (r *pelicanTestRepository) ListQuestions(ctx context.Context, kind string) ([]service.GenerationTestQuestion, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+generationQuestionColumns+` FROM generation_test_questions WHERE test_kind=$1 ORDER BY id`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	questions := []service.GenerationTestQuestion{}
	for rows.Next() {
		q, err := scanGenerationQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (r *pelicanTestRepository) SaveQuestion(ctx context.Context, q service.GenerationTestQuestion) (*service.GenerationTestQuestion, error) {
	if q.ID > 0 {
		out, err := scanGenerationQuestion(r.db.QueryRowContext(ctx, `UPDATE generation_test_questions
			SET name=$3, prompt=$4, expected_answer=$5, enabled=$6, updated_at=NOW()
			WHERE id=$1 AND test_kind=$2 RETURNING `+generationQuestionColumns, q.ID, q.TestKind, q.Name, q.Prompt, q.ExpectedAnswer, q.Enabled))
		return &out, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// The cap is checked under a transaction lock so concurrent creates cannot exceed it.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(751926342)`); err != nil {
		return nil, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_test_questions WHERE test_kind=$1`, q.TestKind).Scan(&count); err != nil {
		return nil, err
	}
	if count >= service.GenerationTestLibraryLimit {
		return nil, fmt.Errorf("%w: library limit is %d", service.ErrInvalidPelicanBatch, service.GenerationTestLibraryLimit)
	}
	out, err := scanGenerationQuestion(tx.QueryRowContext(ctx, `INSERT INTO generation_test_questions(test_kind,name,prompt,expected_answer,enabled)
		VALUES($1,$2,$3,$4,$5) RETURNING `+generationQuestionColumns, q.TestKind, q.Name, q.Prompt, q.ExpectedAnswer, q.Enabled))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *pelicanTestRepository) GetTestPreference(ctx context.Context, userID int64, kind string) (*service.GenerationTestPreference, error) {
	out := &service.GenerationTestPreference{QuestionIDs: []int64{}}
	err := r.db.QueryRowContext(ctx, `SELECT question_ids FROM generation_test_preferences WHERE user_id=$1 AND test_kind=$2`, userID, kind).Scan(pq.Array(&out.QuestionIDs))
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}

func lockGenerationQuestions(ctx context.Context, tx *sql.Tx, kind string, ids []int64) ([]service.GenerationTestQuestion, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+generationQuestionColumns+` FROM generation_test_questions
		WHERE test_kind=$1 AND id=ANY($2) AND enabled ORDER BY id FOR SHARE`, kind, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	questions := []service.GenerationTestQuestion{}
	for rows.Next() {
		q, err := scanGenerationQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(questions) != len(ids) {
		return nil, fmt.Errorf("%w: a selected question is disabled, missing, or belongs to another kind", service.ErrInvalidPelicanBatch)
	}
	return questions, nil
}

func (r *pelicanTestRepository) CreateQuestionBatch(ctx context.Context, batchID string, accountIDs []int64, modelID, kind string, questionIDs []int64, userID int64) ([]service.PelicanTest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = lockGenerationQuestions(ctx, tx, kind, questionIDs); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `INSERT INTO account_pelican_tests
		(batch_id,account_id,model_id,prompt,test_kind,question_id,question_name,expected_answer)
		SELECT $1,a.id,$3,q.prompt,q.test_kind,q.id,q.name,q.expected_answer
		FROM unnest($2::bigint[]) AS a(id) CROSS JOIN generation_test_questions q
		WHERE q.test_kind=$4 AND q.id=ANY($5) AND q.enabled ORDER BY a.id,q.id
		ON CONFLICT (account_id,test_kind,question_id) WHERE status IN ('queued','running') DO NOTHING
		RETURNING `+pelicanTestColumns, batchID, pq.Array(accountIDs), modelID, kind, pq.Array(questionIDs))
	if err != nil {
		return nil, err
	}
	tests := []service.PelicanTest{}
	for rows.Next() {
		test, scanErr := scanPelicanTest(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		tests = append(tests, test)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(tests) > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO generation_test_preferences(user_id,test_kind,question_ids) VALUES($1,$2,$3)
			ON CONFLICT(user_id,test_kind) DO UPDATE SET question_ids=EXCLUDED.question_ids,updated_at=NOW()`, userID, kind, pq.Array(questionIDs))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return tests, nil
}

func (r *pelicanTestRepository) ListLatestBatches(ctx context.Context, accountIDs []int64, kind, batchID string) ([]service.PelicanTest, error) {
	query := `SELECT ` + pelicanSummaryColumns + ` FROM account_pelican_tests WHERE account_id=ANY($1) AND test_kind=$2`
	args := []any{pq.Array(accountIDs), kind}
	if batchID != "" {
		query += ` AND batch_id=$3`
		args = append(args, batchID)
	} else {
		query += ` AND (account_id,batch_id) IN (SELECT DISTINCT ON(account_id) account_id,batch_id
			FROM account_pelican_tests WHERE account_id=ANY($1) AND test_kind=$2 ORDER BY account_id,id DESC)`
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY id DESC LIMIT 2000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PelicanTest{}
	for rows.Next() {
		test, err := scanPelicanTest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, test)
	}
	return out, rows.Err()
}

func (r *pelicanTestRepository) ListBatchHistory(ctx context.Context, accountID int64, page, pageSize int, kind string) (*service.PelicanTestHistory, error) {
	out := &service.PelicanTestHistory{Items: []service.PelicanTest{}, Page: page, PageSize: pageSize}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT batch_id),COUNT(*) FROM account_pelican_tests WHERE account_id=$1 AND test_kind=$2`, accountID, kind).Scan(&out.Total, &out.TotalRecords); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+pelicanSummaryColumns+` FROM account_pelican_tests
		WHERE account_id=$1 AND test_kind=$2 AND batch_id IN (
			SELECT batch_id FROM account_pelican_tests WHERE account_id=$1 AND test_kind=$2
			GROUP BY batch_id ORDER BY MAX(id) DESC LIMIT $3 OFFSET $4)
		ORDER BY id DESC`, accountID, kind, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		test, err := scanPelicanTest(rows)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, test)
	}
	return out, rows.Err()
}
