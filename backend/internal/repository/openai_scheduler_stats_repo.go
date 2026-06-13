package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAISchedulerStatsRepository struct {
	db *sql.DB
}

func NewOpenAISchedulerStatsRepository(db *sql.DB) service.OpenAISchedulerStatsRepository {
	if db == nil {
		return nil
	}
	return &openAISchedulerStatsRepository{db: db}
}

func (r *openAISchedulerStatsRepository) IncrementDailySelection(ctx context.Context, statDate time.Time, groupID int64, accountID int64, selectedAt time.Time) error {
	if r == nil || r.db == nil || groupID <= 0 || accountID <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO openai_scheduler_daily_stats (
			stat_date, group_id, account_id, select_count, last_selected_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, 1, $4, NOW(), NOW())
		ON CONFLICT (stat_date, group_id, account_id) DO UPDATE
		SET select_count = openai_scheduler_daily_stats.select_count + 1,
		    last_selected_at = GREATEST(openai_scheduler_daily_stats.last_selected_at, EXCLUDED.last_selected_at),
		    updated_at = NOW()
	`, statDate, groupID, accountID, selectedAt)
	return err
}

func (r *openAISchedulerStatsRepository) GetDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*service.OpenAISchedulerDailyStats, error) {
	if r == nil || r.db == nil || groupID <= 0 {
		return emptyOpenAISchedulerDailyStats(statDate, groupID), nil
	}
	return r.getDailyStatsWithQuerier(ctx, r.db, statDate, groupID)
}

func (r *openAISchedulerStatsRepository) RecomputeDailyStatsFromUsageLogs(ctx context.Context, statDate time.Time, start time.Time, end time.Time, groupID int64) (*service.OpenAISchedulerDailyStats, error) {
	if r == nil || r.db == nil || groupID <= 0 || !end.After(start) {
		return emptyOpenAISchedulerDailyStats(statDate, groupID), nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM openai_scheduler_daily_stats
		WHERE stat_date = $1 AND group_id = $2
	`, statDate, groupID); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO openai_scheduler_daily_stats (
			stat_date, group_id, account_id, select_count, last_selected_at, created_at, updated_at
		)
		SELECT
			$1::date,
			group_id,
			account_id,
			COUNT(*)::bigint AS select_count,
			MAX(created_at) AS last_selected_at,
			NOW(),
			NOW()
		FROM usage_logs
		WHERE group_id = $2
		  AND account_id > 0
		  AND created_at >= $3
		  AND created_at < $4
		GROUP BY group_id, account_id
	`, statDate, groupID, start, end); err != nil {
		return nil, err
	}

	stats, err := r.getDailyStatsWithQuerier(ctx, tx, statDate, groupID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return stats, nil
}

type openAISchedulerStatsQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *openAISchedulerStatsRepository) getDailyStatsWithQuerier(ctx context.Context, q openAISchedulerStatsQuerier, statDate time.Time, groupID int64) (*service.OpenAISchedulerDailyStats, error) {
	stats := emptyOpenAISchedulerDailyStats(statDate, groupID)
	rows, err := q.QueryContext(ctx, `
		SELECT account_id, select_count, last_selected_at
		FROM openai_scheduler_daily_stats
		WHERE stat_date = $1 AND group_id = $2
		ORDER BY select_count DESC, account_id ASC
	`, statDate, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var item service.OpenAISchedulerAccountDailyStat
		var lastSelected sql.NullTime
		if err := rows.Scan(&item.AccountID, &item.SelectCount, &lastSelected); err != nil {
			return nil, err
		}
		if lastSelected.Valid {
			t := lastSelected.Time
			item.LastSelectedAt = &t
		}
		stats.TotalSelects += item.SelectCount
		stats.Accounts = append(stats.Accounts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stats.TotalSelects > 0 {
		for i := range stats.Accounts {
			stats.Accounts[i].SelectRatio = float64(stats.Accounts[i].SelectCount) / float64(stats.TotalSelects)
		}
	}
	return stats, nil
}

func emptyOpenAISchedulerDailyStats(statDate time.Time, groupID int64) *service.OpenAISchedulerDailyStats {
	return &service.OpenAISchedulerDailyStats{
		Date:    statDate.Format("2006-01-02"),
		GroupID: groupID,
	}
}
