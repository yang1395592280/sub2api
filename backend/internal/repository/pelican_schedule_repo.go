package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const pelicanSummaryColumns = `id, batch_id, account_id, model_id, '' AS prompt, status,
    '' AS response_text, '' AS html, error_message, latency_ms, input_tokens, output_tokens,
    total_tokens, created_at, started_at, finished_at, source_group_id, source_group_name, test_kind, answer, estimated_cost_usd, upstream_rate_multiplier, cost_model_id, question_id, question_name, expected_answer`

func (r *pelicanTestRepository) ListHistory(ctx context.Context, accountID int64, page, pageSize int, kinds ...string) (*service.PelicanTestHistory, error) {
	out := &service.PelicanTestHistory{Items: []service.PelicanTest{}, Page: page, PageSize: pageSize}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_pelican_tests WHERE account_id=$1 AND test_kind=$2`, accountID, service.GenerationTestKind(kinds...)).Scan(&out.Total); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+pelicanSummaryColumns+` FROM account_pelican_tests
        WHERE account_id=$1 AND test_kind=$4 ORDER BY id DESC LIMIT $2 OFFSET $3`, accountID, pageSize, (page-1)*pageSize, service.GenerationTestKind(kinds...))
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

func (r *pelicanTestRepository) GetGroupConfig(ctx context.Context, groupID int64, kinds ...string) (*service.GroupPelicanConfig, error) {
	kind := service.GenerationTestKind(kinds...)
	out := &service.GroupPelicanConfig{TestKind: kind, GroupID: groupID, Prompt: service.GenerationTestPrompt(kind), IntervalMinutes: 30, QuestionIDs: []int64{}}
	err := r.db.QueryRowContext(ctx, `SELECT enabled, model_id, prompt, last_run_at, next_run_at, interval_minutes, question_ids
        FROM group_pelican_test_schedules WHERE group_id=$1 AND test_kind=$2`, groupID, kind).
		Scan(&out.Enabled, &out.ModelID, &out.Prompt, &out.LastRunAt, &out.NextRunAt, &out.IntervalMinutes, pq.Array(&out.QuestionIDs))
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}

func (r *pelicanTestRepository) SaveGroupConfig(ctx context.Context, c service.GroupPelicanConfig) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if c.Enabled && len(c.QuestionIDs) > 0 {
		if _, err = lockGenerationQuestions(ctx, tx, service.GenerationTestKind(c.TestKind), c.QuestionIDs); err != nil {
			return err
		}
	}
	if c.IntervalMinutes == 0 {
		c.IntervalMinutes = 30
	}
	// Preserve the current slot on an unchanged save. Re-enabling or changing the
	// model/prompt creates a fresh due slot, without resetting history.
	_, err = tx.ExecContext(ctx, `INSERT INTO group_pelican_test_schedules
        (group_id, enabled, model_id, prompt, next_run_at, test_kind, interval_minutes, question_ids)
        VALUES ($1,$2,$3,$4,CASE WHEN $2 THEN NOW() ELSE NULL END,$5::text,$6,
          COALESCE($7::bigint[],ARRAY(SELECT id FROM generation_test_questions WHERE test_kind=$5::text AND prompt=$4 ORDER BY id LIMIT 1)))
        ON CONFLICT (group_id, test_kind) DO UPDATE SET enabled=EXCLUDED.enabled, model_id=EXCLUDED.model_id,
        prompt=EXCLUDED.prompt, interval_minutes=EXCLUDED.interval_minutes, question_ids=EXCLUDED.question_ids, updated_at=NOW(), next_run_at=CASE
          WHEN NOT EXCLUDED.enabled THEN NULL
          WHEN NOT group_pelican_test_schedules.enabled
            OR group_pelican_test_schedules.model_id<>EXCLUDED.model_id
            OR group_pelican_test_schedules.prompt<>EXCLUDED.prompt
            OR group_pelican_test_schedules.question_ids<>EXCLUDED.question_ids THEN NOW()
          WHEN group_pelican_test_schedules.interval_minutes<>EXCLUDED.interval_minutes
            THEN COALESCE(group_pelican_test_schedules.last_run_at,NOW())+EXCLUDED.interval_minutes*INTERVAL '1 minute'
          ELSE group_pelican_test_schedules.next_run_at END`, c.GroupID, c.Enabled, c.ModelID, c.Prompt, service.GenerationTestKind(c.TestKind), c.IntervalMinutes, pq.Array(c.QuestionIDs))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *pelicanTestRepository) EnqueueDueGroups(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT s.group_id, s.model_id, s.prompt, g.name, s.test_kind, s.interval_minutes, s.question_ids
        FROM group_pelican_test_schedules s JOIN groups g ON g.id=s.group_id
        WHERE s.enabled AND s.next_run_at<=NOW() AND g.deleted_at IS NULL
        AND g.status='active' AND g.platform='openai'
        ORDER BY s.next_run_at, s.group_id LIMIT 10 FOR UPDATE OF s SKIP LOCKED`)
	if err != nil {
		return err
	}
	type dueGroup struct {
		id                        int64
		model, prompt, name, kind string
		interval                  int
		questions                 []int64
	}
	var groups []dueGroup
	for rows.Next() {
		var g dueGroup
		if err := rows.Scan(&g.id, &g.model, &g.prompt, &g.name, &g.kind, &g.interval, pq.Array(&g.questions)); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, g := range groups {
		// The live membership and the slot advancement commit together. The
		// partial unique account index also prevents overlap with manual jobs.
		if len(g.questions) > 0 {
			_, err = tx.ExecContext(ctx, `INSERT INTO account_pelican_tests
            (batch_id,account_id,model_id,prompt,source_group_id,source_group_name,test_kind,question_id,question_name,expected_answer)
            SELECT $1,a.id,$2,q.prompt,$3,$4,q.test_kind,q.id,q.name,q.expected_answer FROM accounts a
            JOIN account_groups ag ON ag.account_id=a.id CROSS JOIN generation_test_questions q
            WHERE ag.group_id=$3 AND a.deleted_at IS NULL AND a.platform='openai'
              AND a.status='active' AND a.schedulable AND q.test_kind=$5 AND q.enabled AND q.id=ANY($6)
            ORDER BY a.id,q.id
            ON CONFLICT(account_id,test_kind,question_id) WHERE status IN ('queued','running') DO NOTHING`, uuid.NewString(), g.model, g.id, g.name, g.kind, pq.Array(g.questions))
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO account_pelican_tests
            (batch_id,account_id,model_id,prompt,source_group_id,source_group_name,test_kind)
            SELECT $1,a.id,$2,$3,$4,$5,$6 FROM accounts a
            JOIN account_groups ag ON ag.account_id=a.id
            WHERE ag.group_id=$4 AND a.deleted_at IS NULL AND a.platform='openai'
              AND a.status='active' AND a.schedulable
            ORDER BY a.id
            ON CONFLICT (account_id, test_kind, question_id) WHERE status IN ('queued','running') DO NOTHING`, uuid.NewString(), g.model, g.prompt, g.id, g.name, g.kind)
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE group_pelican_test_schedules
            SET last_run_at=NOW(),next_run_at=NOW()+interval_minutes*INTERVAL '1 minute' WHERE group_id=$1 AND test_kind=$2`, g.id, g.kind); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *pelicanTestRepository) ClaimNext(ctx context.Context) (*service.PelicanTest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serialize the count and claim across instances, rather than multiplying
	// paid generation concurrency by the number of application replicas.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(751926341)`); err != nil {
		return nil, err
	}
	test, err := scanPelicanTest(tx.QueryRowContext(ctx, `UPDATE account_pelican_tests
        SET status='running',started_at=NOW() WHERE id=(
          SELECT id FROM account_pelican_tests WHERE status='queued'
          AND (SELECT COUNT(*) FROM account_pelican_tests WHERE status='running')<3
          ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING `+pelicanTestColumns))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &test, nil
}

func (r *pelicanTestRepository) ScheduledTestEnabled(ctx context.Context, test service.PelicanTest) (bool, error) {
	var enabled bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
      SELECT 1 FROM group_pelican_test_schedules s
      JOIN groups g ON g.id=s.group_id JOIN account_groups ag ON ag.group_id=g.id
      JOIN accounts a ON a.id=ag.account_id
      WHERE s.group_id=$1 AND s.test_kind=$5 AND s.enabled AND s.model_id=$2
      AND (($6::bigint=0 AND cardinality(s.question_ids)=0 AND s.prompt=$3) OR
        ($6::bigint=ANY(s.question_ids) AND EXISTS(SELECT 1 FROM generation_test_questions q WHERE q.id=$6 AND q.test_kind=$5 AND q.enabled)))
      AND g.deleted_at IS NULL AND g.status='active' AND g.platform='openai'
      AND a.id=$4 AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable
      AND a.platform='openai')`, test.SourceGroupID, test.ModelID, test.Prompt, test.AccountID, service.GenerationTestKind(test.TestKind), test.QuestionID).Scan(&enabled)
	return enabled, err
}
