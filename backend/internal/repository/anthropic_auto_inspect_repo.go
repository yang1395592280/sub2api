package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type anthropicAutoInspectDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type anthropicAutoInspectRepository struct {
	db anthropicAutoInspectDB
}

func NewAnthropicAutoInspectRepository(db *sql.DB) service.AnthropicAutoInspectRepository {
	return &anthropicAutoInspectRepository{db: db}
}

func (r *anthropicAutoInspectRepository) CreateBatch(ctx context.Context, input service.CreateAnthropicAutoInspectBatchInput) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO anthropic_auto_inspect_batches (
			trigger_source,
			status,
			started_at
		)
		VALUES ($1, $2, $3)
		RETURNING id
	`, input.TriggerSource, input.Status, input.StartedAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *anthropicAutoInspectRepository) CreateSkippedBatch(ctx context.Context, input service.CreateAnthropicAutoInspectSkippedBatchInput) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO anthropic_auto_inspect_batches (
			trigger_source,
			status,
			skip_reason,
			started_at,
			finished_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.TriggerSource, service.AnthropicAutoInspectBatchStatusSkipped, input.SkipReason, input.StartedAt, input.FinishedAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *anthropicAutoInspectRepository) CreateLog(ctx context.Context, log service.AnthropicAutoInspectLog) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO anthropic_auto_inspect_logs (
			batch_id,
			account_id,
			account_name_snapshot,
			platform,
			account_type,
			result,
			skip_reason,
			response_text,
			error_message,
			rate_limit_reset_at,
			temp_unschedulable_until,
			schedulable_changed,
			started_at,
			finished_at,
			latency_ms
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		log.BatchID,
		log.AccountID,
		log.AccountNameSnapshot,
		log.Platform,
		log.AccountType,
		log.Result,
		log.SkipReason,
		log.ResponseText,
		log.ErrorMessage,
		log.RateLimitResetAt,
		log.TempUnschedulableUntil,
		log.SchedulableChanged,
		log.StartedAt,
		log.FinishedAt,
		log.LatencyMs,
	)
	return err
}

func (r *anthropicAutoInspectRepository) CompleteBatch(ctx context.Context, batchID int64, stats service.AnthropicAutoInspectBatchStats, finishedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE anthropic_auto_inspect_batches
		SET
			status = $2,
			total_accounts = $3,
			processed_accounts = $4,
			success_count = $5,
			rate_limited_count = $6,
			error_count = $7,
			skipped_count = $8,
			finished_at = $9
		WHERE id = $1
	`,
		batchID,
		service.AnthropicAutoInspectBatchStatusCompleted,
		stats.TotalAccounts,
		stats.ProcessedAccounts,
		stats.SuccessCount,
		stats.RateLimitedCount,
		stats.ErrorCount,
		stats.SkippedCount,
		finishedAt,
	)
	return err
}

func (r *anthropicAutoInspectRepository) MarkBatchFailed(ctx context.Context, batchID int64, stats service.AnthropicAutoInspectBatchStats, finishedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE anthropic_auto_inspect_batches
		SET
			status = $2,
			total_accounts = $3,
			processed_accounts = $4,
			success_count = $5,
			rate_limited_count = $6,
			error_count = $7,
			skipped_count = $8,
			finished_at = $9
		WHERE id = $1
	`,
		batchID,
		service.AnthropicAutoInspectBatchStatusFailed,
		stats.TotalAccounts,
		stats.ProcessedAccounts,
		stats.SuccessCount,
		stats.RateLimitedCount,
		stats.ErrorCount,
		stats.SkippedCount,
		finishedAt,
	)
	return err
}

func (r *anthropicAutoInspectRepository) ListLogs(
	ctx context.Context,
	params pagination.PaginationParams,
	filter service.AnthropicAutoInspectLogFilter,
) ([]service.AnthropicAutoInspectLog, *pagination.PaginationResult, error) {
	where := []string{"1=1"}
	args := make([]any, 0, 4)

	if filter.Result != "" {
		args = append(args, filter.Result)
		where = append(where, fmt.Sprintf("result = $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf(`(
			account_name_snapshot ILIKE $%d OR
			response_text ILIKE $%d OR
			error_message ILIKE $%d
		)`, len(args), len(args), len(args)))
	}
	if filter.StartedFrom != nil {
		args = append(args, filter.StartedFrom.UTC())
		where = append(where, fmt.Sprintf("started_at >= $%d", len(args)))
	}
	if filter.StartedTo != nil {
		args = append(args, filter.StartedTo.UTC())
		where = append(where, fmt.Sprintf("started_at <= $%d", len(args)))
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM anthropic_auto_inspect_logs
		WHERE %s
	`, strings.Join(where, " AND "))

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	listQuery := fmt.Sprintf(`
		SELECT
			id,
			batch_id,
			account_id,
			account_name_snapshot,
			platform,
			account_type,
			result,
			skip_reason,
			response_text,
			error_message,
			rate_limit_reset_at,
			temp_unschedulable_until,
			schedulable_changed,
			started_at,
			finished_at,
			latency_ms,
			created_at
		FROM anthropic_auto_inspect_logs
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), limitArg, offsetArg)

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	logs := make([]service.AnthropicAutoInspectLog, 0)
	for rows.Next() {
		log, scanErr := scanAnthropicAutoInspectLog(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	pageSize := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	pages := 0
	if pageSize > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return logs, &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}, nil
}

func (r *anthropicAutoInspectRepository) ListBatches(ctx context.Context, params pagination.PaginationParams) ([]service.AnthropicAutoInspectBatch, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM anthropic_auto_inspect_batches`).Scan(&total); err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			trigger_source,
			status,
			skip_reason,
			total_accounts,
			processed_accounts,
			success_count,
			rate_limited_count,
			error_count,
			skipped_count,
			started_at,
			finished_at,
			created_at
		FROM anthropic_auto_inspect_batches
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	batches := make([]service.AnthropicAutoInspectBatch, 0)
	for rows.Next() {
		var batch service.AnthropicAutoInspectBatch
		if err := rows.Scan(
			&batch.ID,
			&batch.TriggerSource,
			&batch.Status,
			&batch.SkipReason,
			&batch.TotalAccounts,
			&batch.ProcessedAccounts,
			&batch.SuccessCount,
			&batch.RateLimitedCount,
			&batch.ErrorCount,
			&batch.SkippedCount,
			&batch.StartedAt,
			&batch.FinishedAt,
			&batch.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	pageSize := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	pages := 0
	if pageSize > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return batches, &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}, nil
}

func scanAnthropicAutoInspectLog(row scannable) (service.AnthropicAutoInspectLog, error) {
	var log service.AnthropicAutoInspectLog
	err := row.Scan(
		&log.ID,
		&log.BatchID,
		&log.AccountID,
		&log.AccountNameSnapshot,
		&log.Platform,
		&log.AccountType,
		&log.Result,
		&log.SkipReason,
		&log.ResponseText,
		&log.ErrorMessage,
		&log.RateLimitResetAt,
		&log.TempUnschedulableUntil,
		&log.SchedulableChanged,
		&log.StartedAt,
		&log.FinishedAt,
		&log.LatencyMs,
		&log.CreatedAt,
	)
	return log, err
}
