package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const pelicanSummaryColumns = `id, batch_id, account_id, model_id, '' AS prompt, status,
    '' AS response_text, '' AS html, error_message, latency_ms, input_tokens, output_tokens,
    total_tokens, created_at, started_at, finished_at, source_group_id, source_group_name`

func (r *pelicanTestRepository) ListHistory(ctx context.Context, accountID int64, page, pageSize int) (*service.PelicanTestHistory, error) {
	out := &service.PelicanTestHistory{Items: []service.PelicanTest{}, Page: page, PageSize: pageSize}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_pelican_tests WHERE account_id=$1`, accountID).Scan(&out.Total); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+pelicanSummaryColumns+` FROM account_pelican_tests
        WHERE account_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, accountID, pageSize, (page-1)*pageSize)
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

func (r *pelicanTestRepository) GetGroupConfig(ctx context.Context, groupID int64) (*service.GroupPelicanConfig, error) {
	out := &service.GroupPelicanConfig{GroupID: groupID, Prompt: service.DefaultPelicanTestPrompt, IntervalMinutes: 30}
	err := r.db.QueryRowContext(ctx, `SELECT enabled, model_id, prompt, last_run_at, next_run_at
        FROM group_pelican_test_schedules WHERE group_id=$1`, groupID).
		Scan(&out.Enabled, &out.ModelID, &out.Prompt, &out.LastRunAt, &out.NextRunAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}

func (r *pelicanTestRepository) SaveGroupConfig(ctx context.Context, c service.GroupPelicanConfig) error {
	// Preserve the current slot on an unchanged save. Re-enabling or changing the
	// model/prompt creates a fresh due slot, without resetting history.
	_, err := r.db.ExecContext(ctx, `INSERT INTO group_pelican_test_schedules
        (group_id, enabled, model_id, prompt, next_run_at)
        VALUES ($1,$2,$3,$4,CASE WHEN $2 THEN NOW() ELSE NULL END)
        ON CONFLICT (group_id) DO UPDATE SET enabled=EXCLUDED.enabled, model_id=EXCLUDED.model_id,
        prompt=EXCLUDED.prompt, updated_at=NOW(), next_run_at=CASE
          WHEN NOT EXCLUDED.enabled THEN NULL
          WHEN NOT group_pelican_test_schedules.enabled
            OR group_pelican_test_schedules.model_id<>EXCLUDED.model_id
            OR group_pelican_test_schedules.prompt<>EXCLUDED.prompt THEN NOW()
          ELSE group_pelican_test_schedules.next_run_at END`, c.GroupID, c.Enabled, c.ModelID, c.Prompt)
	return err
}

func (r *pelicanTestRepository) EnqueueDueGroups(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT s.group_id, s.model_id, s.prompt, g.name
        FROM group_pelican_test_schedules s JOIN groups g ON g.id=s.group_id
        WHERE s.enabled AND s.next_run_at<=NOW() AND g.deleted_at IS NULL
        AND g.status='active' AND g.platform='openai'
        ORDER BY s.next_run_at, s.group_id LIMIT 10 FOR UPDATE OF s SKIP LOCKED`)
	if err != nil {
		return err
	}
	type dueGroup struct {
		id                  int64
		model, prompt, name string
	}
	var groups []dueGroup
	for rows.Next() {
		var g dueGroup
		if err := rows.Scan(&g.id, &g.model, &g.prompt, &g.name); err != nil {
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
		_, err = tx.ExecContext(ctx, `INSERT INTO account_pelican_tests
            (batch_id,account_id,model_id,prompt,source_group_id,source_group_name)
            SELECT $1,a.id,$2,$3,$4,$5 FROM accounts a
            JOIN account_groups ag ON ag.account_id=a.id
            WHERE ag.group_id=$4 AND a.deleted_at IS NULL AND a.platform='openai'
              AND a.status='active' AND a.schedulable
            ORDER BY a.id
            ON CONFLICT (account_id) WHERE status IN ('queued','running') DO NOTHING`, uuid.NewString(), g.model, g.prompt, g.id, g.name)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE group_pelican_test_schedules
            SET last_run_at=NOW(),next_run_at=NOW()+INTERVAL '30 minutes' WHERE group_id=$1`, g.id); err != nil {
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
      WHERE s.group_id=$1 AND s.enabled AND s.model_id=$2 AND s.prompt=$3
      AND g.deleted_at IS NULL AND g.status='active' AND g.platform='openai'
      AND a.id=$4 AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable
      AND a.platform='openai')`, test.SourceGroupID, test.ModelID, test.Prompt, test.AccountID).Scan(&enabled)
	return enabled, err
}
