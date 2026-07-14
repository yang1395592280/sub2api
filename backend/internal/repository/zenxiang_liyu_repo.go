package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type zenxiangLiyuRepository struct {
	client *ent.Client
	db     *sql.DB
}

const zenxiangLiyuAmountScale = 100_000_000

func NewZenxiangLiyuRepository(client *ent.Client, sqlDB *sql.DB) service.ZenxiangLiyuRepository {
	return &zenxiangLiyuRepository{client: client, db: sqlDB}
}

func (r *zenxiangLiyuRepository) GetSettings(ctx context.Context) (*service.ZenxiangLiyuSettings, error) {
	settings := &service.ZenxiangLiyuSettings{}
	err := r.db.QueryRowContext(ctx, `
		SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,
		       COALESCE(ticket_usage_threshold, 5), COALESCE(daily_ticket_limit, 3),
		       COALESCE(unit_sale_price, 0.1), COALESCE(unit_cost_price, 0.05),
		       COALESCE(lucky_coin_enabled, TRUE), COALESCE(lucky_coin_double_probability, 50)
		FROM zenxiang_liyu_settings WHERE id = 1`,
	).Scan(
		&settings.GlobalEnabled, &settings.TicketAmount, &settings.MinimumBalance, &settings.DailyPlayLimit,
		&settings.TicketUsageThreshold, &settings.DailyTicketLimit, &settings.UnitSalePrice, &settings.UnitCostPrice,
		&settings.LuckyCoinEnabled, &settings.LuckyCoinProbability,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *zenxiangLiyuRepository) UpdateSettings(ctx context.Context, settings service.ZenxiangLiyuSettings) (*service.ZenxiangLiyuSettings, error) {
	updated := &service.ZenxiangLiyuSettings{}
	err := r.db.QueryRowContext(ctx, `
		UPDATE zenxiang_liyu_settings
		SET global_enabled = $1, ticket_amount = $2, minimum_balance = $3, daily_play_limit = $4,
		    ticket_usage_threshold = $5, daily_ticket_limit = $6,
		    unit_sale_price = $7, unit_cost_price = $8,
		    lucky_coin_enabled = $9, lucky_coin_double_probability = $10, updated_at = NOW()
		WHERE id = 1
		RETURNING global_enabled, ticket_amount, minimum_balance, daily_play_limit,
		          ticket_usage_threshold, daily_ticket_limit, unit_sale_price, unit_cost_price,
		          lucky_coin_enabled, lucky_coin_double_probability`,
		settings.GlobalEnabled, settings.TicketAmount, settings.MinimumBalance, settings.DailyPlayLimit,
		settings.TicketUsageThreshold, settings.DailyTicketLimit, settings.UnitSalePrice, settings.UnitCostPrice,
		settings.LuckyCoinEnabled, settings.LuckyCoinProbability,
	).Scan(
		&updated.GlobalEnabled, &updated.TicketAmount, &updated.MinimumBalance, &updated.DailyPlayLimit,
		&updated.TicketUsageThreshold, &updated.DailyTicketLimit, &updated.UnitSalePrice, &updated.UnitCostPrice,
		&updated.LuckyCoinEnabled, &updated.LuckyCoinProbability,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *zenxiangLiyuRepository) ListPrizes(ctx context.Context) ([]service.ZenxiangLiyuPrize, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, reward_amount, probability, enabled, sort_order
		FROM zenxiang_liyu_prizes ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanZenxiangLiyuPrizes(rows)
}

func (r *zenxiangLiyuRepository) SavePrize(ctx context.Context, prize service.ZenxiangLiyuPrize) (*service.ZenxiangLiyuPrize, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = lockZenxiangLiyuSettingsForUpdate(ctx, tx); err != nil {
		return nil, err
	}

	if prize.ID == 0 {
		created := &service.ZenxiangLiyuPrize{}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO zenxiang_liyu_prizes (name, reward_amount, probability, enabled, sort_order)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, reward_amount, probability, enabled, sort_order`,
			prize.Name, prize.RewardAmount, prize.Probability, prize.Enabled, prize.SortOrder,
		).Scan(&created.ID, &created.Name, &created.RewardAmount, &created.Probability, &created.Enabled, &created.SortOrder)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return created, nil
	}

	updated := &service.ZenxiangLiyuPrize{}
	err = tx.QueryRowContext(ctx, `
		UPDATE zenxiang_liyu_prizes
		SET name = $1, reward_amount = $2, probability = $3, enabled = $4, sort_order = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING id, name, reward_amount, probability, enabled, sort_order`,
		prize.Name, prize.RewardAmount, prize.Probability, prize.Enabled, prize.SortOrder, prize.ID,
	).Scan(&updated.ID, &updated.Name, &updated.RewardAmount, &updated.Probability, &updated.Enabled, &updated.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

// SavePrizes atomically persists a submitted complete prize configuration.
func (r *zenxiangLiyuRepository) SavePrizes(ctx context.Context, prizes []service.ZenxiangLiyuPrize) (_ []service.ZenxiangLiyuPrize, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = lockZenxiangLiyuSettingsForUpdate(ctx, tx); err != nil {
		return nil, err
	}

	saved := make([]service.ZenxiangLiyuPrize, 0, len(prizes))
	for _, prize := range prizes {
		var stored service.ZenxiangLiyuPrize
		if prize.ID == 0 {
			err = tx.QueryRowContext(ctx, `
				INSERT INTO zenxiang_liyu_prizes (name, reward_amount, probability, enabled, sort_order)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id, name, reward_amount, probability, enabled, sort_order`,
				prize.Name, prize.RewardAmount, prize.Probability, prize.Enabled, prize.SortOrder,
			).Scan(&stored.ID, &stored.Name, &stored.RewardAmount, &stored.Probability, &stored.Enabled, &stored.SortOrder)
		} else {
			err = tx.QueryRowContext(ctx, `
				UPDATE zenxiang_liyu_prizes
				SET name = $1, reward_amount = $2, probability = $3, enabled = $4, sort_order = $5, updated_at = NOW()
				WHERE id = $6
				RETURNING id, name, reward_amount, probability, enabled, sort_order`,
				prize.Name, prize.RewardAmount, prize.Probability, prize.Enabled, prize.SortOrder, prize.ID,
			).Scan(&stored.ID, &stored.Name, &stored.RewardAmount, &stored.Probability, &stored.Enabled, &stored.SortOrder)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrZenxiangLiyuInvalidSettings
		}
		if err != nil {
			return nil, err
		}
		saved = append(saved, stored)
	}
	if err = disableOmittedZenxiangLiyuPrizes(ctx, tx, saved); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return saved, nil
}

func (r *zenxiangLiyuRepository) DeletePrize(ctx context.Context, id int64) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = lockZenxiangLiyuSettingsForUpdate(ctx, tx); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM zenxiang_liyu_prizes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		err = service.ErrZenxiangLiyuInvalidSettings
		return err
	}
	return tx.Commit()
}

func (r *zenxiangLiyuRepository) IsUserGranted(ctx context.Context, userID int64) (bool, error) {
	var granted bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM zenxiang_liyu_user_grants WHERE user_id = $1 AND enabled = TRUE)`, userID,
	).Scan(&granted)
	return granted, err
}

func (r *zenxiangLiyuRepository) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	var balance float64
	err := r.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrUserNotFound
	}
	return balance, err
}

func (r *zenxiangLiyuRepository) CountUserPlaysOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT GREATEST(
			COUNT(*) - COALESCE((SELECT reset_count FROM zenxiang_liyu_daily_resets WHERE user_id = $1 AND play_date = $2), 0),
			0
		)
		FROM zenxiang_liyu_records WHERE user_id = $1 AND created_at >= $3 AND created_at < $4`, userID, playDate, start, end,
	).Scan(&count)
	return count, err
}

func (r *zenxiangLiyuRepository) GetUserUsageAmountOnDate(ctx context.Context, userID int64, playDate time.Time) (float64, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var amount float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(actual_cost), 0)
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`, userID, start, end,
	).Scan(&amount)
	return amount, err
}

func (r *zenxiangLiyuRepository) HasUserFreePlayOnDate(ctx context.Context, userID int64, playDate time.Time) (bool, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var used bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM zenxiang_liyu_records
			WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND ticket_amount = 0
		)`, userID, start, end,
	).Scan(&used)
	return used, err
}

func (r *zenxiangLiyuRepository) ListUserRecords(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]service.ZenxiangLiyuRecord, int, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zenxiang_liyu_records WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`, userID, start, end).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, request_id, ticket_amount, reward_amount, user_net_amount,
		       COALESCE(lucky_coin_played, FALSE), COALESCE(lucky_coin_outcome, ''),
		       COALESCE(lucky_coin_adjustment, 0), balance_after_lucky,
		       prize_id, prize_name_snapshot, probability_snapshot, created_at
		FROM zenxiang_liyu_records
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at DESC, id DESC
		LIMIT $4 OFFSET $5`, userID, start, end, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	records := make([]service.ZenxiangLiyuRecord, 0)
	for rows.Next() {
		var record service.ZenxiangLiyuRecord
		if err := rows.Scan(
			&record.ID, &record.RequestID, &record.TicketAmount, &record.RewardAmount, &record.UserNetAmount,
			&record.LuckyCoinPlayed, &record.LuckyCoinOutcome, &record.LuckyCoinAdjustment, &record.BalanceAfterLucky,
			&record.PrizeID, &record.PrizeName, &record.Probability, &record.PlayedAt,
		); err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}
	return records, total, rows.Err()
}

func (r *zenxiangLiyuRepository) GetUserDailySummary(ctx context.Context, userID int64, playDate time.Time) (*service.ZenxiangLiyuDailySummary, error) {
	summary := &service.ZenxiangLiyuDailySummary{PlayDate: playDate}
	start, end := zenxiangLiyuUsageWindow(playDate)
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(ticket_amount), 0),
		       COALESCE(SUM(reward_amount + COALESCE(lucky_coin_adjustment, 0)), 0),
		       COALESCE(SUM(user_net_amount), 0)
		FROM zenxiang_liyu_records
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`, userID, start, end).Scan(&summary.PlayCount, &summary.TicketAmount, &summary.RewardAmount, &summary.UserNetAmount)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func (r *zenxiangLiyuRepository) ListGrants(ctx context.Context, page, pageSize int) ([]service.ZenxiangLiyuGrant, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zenxiang_liyu_user_grants`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT g.user_id, u.email, g.enabled, g.granted_by, g.notes, g.created_at, g.updated_at FROM zenxiang_liyu_user_grants g JOIN users u ON u.id = g.user_id ORDER BY g.updated_at DESC, g.user_id DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	grants := make([]service.ZenxiangLiyuGrant, 0)
	for rows.Next() {
		var grant service.ZenxiangLiyuGrant
		if err := rows.Scan(&grant.UserID, &grant.UserEmail, &grant.Enabled, &grant.GrantedBy, &grant.Notes, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, 0, err
		}
		grants = append(grants, grant)
	}
	return grants, total, rows.Err()
}

func (r *zenxiangLiyuRepository) SaveGrant(ctx context.Context, grant service.ZenxiangLiyuGrant) (*service.ZenxiangLiyuGrant, error) {
	stored := &service.ZenxiangLiyuGrant{}
	err := r.db.QueryRowContext(ctx, `INSERT INTO zenxiang_liyu_user_grants (user_id, enabled, granted_by, notes) VALUES ($1, $2, $3, $4) ON CONFLICT (user_id) DO UPDATE SET enabled = EXCLUDED.enabled, granted_by = EXCLUDED.granted_by, notes = EXCLUDED.notes, updated_at = NOW() RETURNING user_id, enabled, granted_by, notes, created_at, updated_at`, grant.UserID, grant.Enabled, grant.GrantedBy, grant.Notes).Scan(&stored.UserID, &stored.Enabled, &stored.GrantedBy, &stored.Notes, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (r *zenxiangLiyuRepository) DeleteGrant(ctx context.Context, userID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM zenxiang_liyu_user_grants WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrZenxiangLiyuInvalidSettings
	}
	return nil
}

func (r *zenxiangLiyuRepository) CountGiftedTicketsOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(ticket_count), 0)
		FROM zenxiang_liyu_ticket_gifts
		WHERE user_id = $1 AND play_date = $2`, userID, playDate,
	).Scan(&count)
	return count, err
}

func (r *zenxiangLiyuRepository) SyncTicketBalance(ctx context.Context, userID int64, playDate time.Time, settings service.ZenxiangLiyuSettings) (_ int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	balance, err := syncZenxiangLiyuTicketBalance(ctx, tx, userID, playDate, &settings)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return balance, nil
}

func (r *zenxiangLiyuRepository) GiftTickets(ctx context.Context, gift service.ZenxiangLiyuTicketGift) (*service.ZenxiangLiyuTicketGift, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userEmail string
	err = tx.QueryRowContext(ctx, `
		SELECT email FROM users
		WHERE id = $1 AND role = $2 AND status = $3 AND deleted_at IS NULL
		FOR SHARE`, gift.UserID, service.RoleUser, service.StatusActive,
	).Scan(&userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	stored := &service.ZenxiangLiyuTicketGift{UserEmail: userEmail}
	inserted := true
	err = tx.QueryRowContext(ctx, `
		INSERT INTO zenxiang_liyu_ticket_gifts (request_id, user_id, play_date, ticket_count, granted_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (request_id) DO NOTHING
		RETURNING id, request_id, user_id, play_date, ticket_count, granted_by, notes, created_at, updated_at`,
		gift.RequestID, gift.UserID, gift.PlayDate, gift.TicketCount, gift.GrantedBy, gift.Notes,
	).Scan(
		&stored.ID, &stored.RequestID, &stored.UserID, &stored.PlayDate, &stored.TicketCount,
		&stored.GrantedBy, &stored.Notes, &stored.CreatedAt, &stored.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		inserted = false
		err = tx.QueryRowContext(ctx, `
			SELECT g.id, g.request_id, g.user_id, u.email, g.play_date, g.ticket_count, g.granted_by, g.notes, g.created_at, g.updated_at
			FROM zenxiang_liyu_ticket_gifts g
			JOIN users u ON u.id = g.user_id
			WHERE g.request_id = $1`, gift.RequestID,
		).Scan(
			&stored.ID, &stored.RequestID, &stored.UserID, &stored.UserEmail, &stored.PlayDate, &stored.TicketCount,
			&stored.GrantedBy, &stored.Notes, &stored.CreatedAt, &stored.UpdatedAt,
		)
	}
	if err != nil {
		return nil, err
	}
	if inserted {
		balance, balanceErr := lockAndReconcileZenxiangLiyuTicketWallet(ctx, tx, gift.UserID, gift.PlayDate)
		if balanceErr != nil {
			return nil, balanceErr
		}
		acceptedTickets := min(service.ZenxiangLiyuTicketCapacity-balance, gift.TicketCount)
		if acceptedTickets > 0 {
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO zenxiang_liyu_ticket_batches (
					user_id, source_type, source_key, granted_count, remaining_count, expires_at
				) VALUES ($1, 'gift', $2, $3, $3, $4)`,
				gift.UserID, gift.RequestID, acceptedTickets, zenxiangLiyuTicketExpiry(gift.PlayDate),
			); err != nil {
				return nil, err
			}
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE zenxiang_liyu_ticket_wallets
			SET balance = balance + $1, updated_at = NOW()
			WHERE user_id = $2`, acceptedTickets, gift.UserID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (r *zenxiangLiyuRepository) GetOverviewStats(ctx context.Context) (*service.ZenxiangLiyuOverviewStats, error) {
	stats := &service.ZenxiangLiyuOverviewStats{}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(system_revenue), 0), COALESCE(SUM(system_expense), 0), COALESCE(SUM(system_profit), 0), COUNT(DISTINCT user_id) FROM zenxiang_liyu_records`).Scan(&stats.TotalPlays, &stats.TotalRevenue, &stats.TotalExpense, &stats.NetProfit, &stats.ParticipatingUsers)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *zenxiangLiyuRepository) ListUserStats(ctx context.Context, page, pageSize int, playDate time.Time) ([]service.ZenxiangLiyuUserStats, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM zenxiang_liyu_records WHERE play_date = $1`, playDate).Scan(&total); err != nil {
		return nil, 0, err
	}
	start, end := zenxiangLiyuUsageWindow(playDate)
	rows, err := r.db.QueryContext(ctx, `
		WITH usage_amount AS (
			SELECT user_id, COALESCE(SUM(actual_cost), 0) AS amount
			FROM usage_logs
			WHERE created_at >= $2 AND created_at < $3
			GROUP BY user_id
		)
		SELECT r.user_id, u.email, u.balance, COALESCE(ua.amount, 0), COUNT(*),
			COALESCE(SUM(r.ticket_amount), 0),
			COALESCE(SUM(r.reward_amount + COALESCE(r.lucky_coin_adjustment, 0)), 0),
			COALESCE(SUM(r.user_net_amount), 0)
		FROM zenxiang_liyu_records r
		JOIN users u ON u.id = r.user_id
		LEFT JOIN usage_amount ua ON ua.user_id = r.user_id
		WHERE r.play_date = $1
		GROUP BY r.user_id, u.email, u.balance, ua.amount
		ORDER BY COUNT(*) DESC, r.user_id DESC
		LIMIT $4 OFFSET $5`, playDate, start, end, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	stats := make([]service.ZenxiangLiyuUserStats, 0)
	for rows.Next() {
		var stat service.ZenxiangLiyuUserStats
		if err := rows.Scan(&stat.UserID, &stat.UserEmail, &stat.Balance, &stat.UsageAmount, &stat.PlayCount, &stat.TicketAmount, &stat.RewardAmount, &stat.UserNetAmount); err != nil {
			return nil, 0, err
		}
		stats = append(stats, stat)
	}
	return stats, total, rows.Err()
}

func (r *zenxiangLiyuRepository) ListPrizeStats(ctx context.Context) ([]service.ZenxiangLiyuPrizeStats, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT prize_id, prize_name_snapshot, COUNT(*), COALESCE(SUM(reward_amount), 0), MAX(probability_snapshot) FROM zenxiang_liyu_records GROUP BY prize_id, prize_name_snapshot ORDER BY COUNT(*) DESC, prize_name_snapshot`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := make([]service.ZenxiangLiyuPrizeStats, 0)
	for rows.Next() {
		var stat service.ZenxiangLiyuPrizeStats
		if err := rows.Scan(&stat.PrizeID, &stat.PrizeName, &stat.HitCount, &stat.RewardAmount, &stat.Probability); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

func (r *zenxiangLiyuRepository) ListPeriodStats(ctx context.Context, period string) ([]service.ZenxiangLiyuPeriodStats, error) {
	trunc := "day"
	switch period {
	case "week":
		trunc = "week"
	case "month":
		trunc = "month"
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH base AS (
			SELECT date_trunc($1, created_at AT TIME ZONE 'Asia/Shanghai')::date AS period_start,
				user_id, ticket_amount, reward_amount + COALESCE(lucky_coin_adjustment, 0) AS reward_amount,
				user_net_amount, system_revenue, system_expense, system_profit,
				prize_name_snapshot
			FROM zenxiang_liyu_records
		),
		rollup AS (
			SELECT period_start, COUNT(*) AS play_count, COUNT(DISTINCT user_id) AS participant_count,
				COUNT(*) AS tickets_used,
				COALESCE(SUM(ticket_amount), 0) AS ticket_amount,
				COALESCE(SUM(reward_amount), 0) AS reward_amount,
				COALESCE(AVG(reward_amount), 0) AS average_reward,
				COALESCE(SUM(user_net_amount), 0) AS user_net_amount,
				COALESCE(SUM(system_revenue), 0) AS system_revenue,
				COALESCE(SUM(system_expense), 0) AS system_expense,
				COALESCE(SUM(system_profit), 0) AS system_profit
			FROM base GROUP BY period_start
		),
		usage_rollup AS (
			SELECT date_trunc($1, created_at AT TIME ZONE 'Asia/Shanghai')::date AS period_start,
				COALESCE(SUM(actual_cost), 0) AS usage_amount
			FROM usage_logs
			GROUP BY period_start
		),
		top_prize AS (
			SELECT period_start, prize_name_snapshot, COUNT(*) AS hit_count,
				ROW_NUMBER() OVER (PARTITION BY period_start ORDER BY COUNT(*) DESC, prize_name_snapshot) AS rn
			FROM base GROUP BY period_start, prize_name_snapshot
		)
		SELECT r.period_start, r.play_count, r.participant_count, COALESCE(u.usage_amount, 0), r.tickets_used,
			r.ticket_amount, r.reward_amount, r.average_reward,
			r.user_net_amount, r.system_revenue, r.system_expense, r.system_profit,
			COALESCE(t.prize_name_snapshot, ''), COALESCE(t.hit_count, 0)
		FROM rollup r
		LEFT JOIN usage_rollup u ON u.period_start = r.period_start
		LEFT JOIN top_prize t ON t.period_start = r.period_start AND t.rn = 1
		ORDER BY r.period_start DESC
		LIMIT 120`, trunc)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := make([]service.ZenxiangLiyuPeriodStats, 0)
	for rows.Next() {
		var stat service.ZenxiangLiyuPeriodStats
		if err := rows.Scan(
			&stat.PeriodStart, &stat.PlayCount, &stat.ParticipantCount, &stat.UsageAmount, &stat.TicketsUsed,
			&stat.TicketAmount, &stat.RewardAmount, &stat.AverageReward,
			&stat.UserNetAmount, &stat.SystemRevenue, &stat.SystemExpense, &stat.SystemProfit,
			&stat.MostHitPrizeName, &stat.MostHitPrizeCount,
		); err != nil {
			return nil, err
		}
		stat.PeriodLabel = stat.PeriodStart.Format("2006-01-02")
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

func (r *zenxiangLiyuRepository) ResetUserDailyPlays(ctx context.Context, userID int64, playDate time.Time, resetBy *int64, notes string) (_ int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var playCount, previousResetCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM zenxiang_liyu_records WHERE user_id = $1 AND play_date = $2`, userID, playDate).Scan(&playCount)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(reset_count, 0)
		FROM zenxiang_liyu_daily_resets
		WHERE user_id = $1 AND play_date = $2`, userID, playDate,
	).Scan(&previousResetCount)
	if errors.Is(err, sql.ErrNoRows) {
		previousResetCount = 0
		err = nil
	}
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO zenxiang_liyu_daily_resets (user_id, play_date, reset_count, reset_by, notes)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, play_date)
		DO UPDATE SET reset_count = EXCLUDED.reset_count, reset_by = EXCLUDED.reset_by, notes = EXCLUDED.notes, updated_at = NOW()`,
		userID, playDate, playCount, resetBy, notes,
	)
	if err != nil {
		return 0, err
	}
	refundedTickets := max(0, playCount-previousResetCount)
	if refundedTickets > 0 {
		balance, balanceErr := lockAndReconcileZenxiangLiyuTicketWallet(ctx, tx, userID, playDate)
		if balanceErr != nil {
			return 0, balanceErr
		}
		acceptedTickets := min(service.ZenxiangLiyuTicketCapacity-balance, refundedTickets)
		if acceptedTickets > 0 {
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO zenxiang_liyu_ticket_batches (
					user_id, source_type, source_key, granted_count, remaining_count, expires_at
				) VALUES ($1, 'reset', $2, $3, $3, $4)
				ON CONFLICT (user_id, source_type, source_key)
				DO UPDATE SET granted_count = zenxiang_liyu_ticket_batches.granted_count + EXCLUDED.granted_count,
				              remaining_count = zenxiang_liyu_ticket_batches.remaining_count + EXCLUDED.remaining_count,
				              expires_at = EXCLUDED.expires_at,
				              updated_at = NOW()`,
				userID, playDate.Format("2006-01-02"), acceptedTickets, zenxiangLiyuTicketExpiry(playDate),
			); err != nil {
				return 0, err
			}
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE zenxiang_liyu_ticket_wallets
			SET balance = balance + $1, updated_at = NOW()
			WHERE user_id = $2`, acceptedTickets, userID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return playCount, nil
}

func (r *zenxiangLiyuRepository) Play(ctx context.Context, cmd service.ZenxiangLiyuPlayCommand) (_ *service.ZenxiangLiyuPlayResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if existing, lookupErr := findZenxiangLiyuRecord(ctx, tx, cmd.UserID, cmd.RequestID); lookupErr == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}

	settings, err := getZenxiangLiyuSettingsForPlay(ctx, tx)
	if err != nil {
		return nil, err
	}
	if settings.TicketAmount < 0 || settings.MinimumBalance < 0 || settings.DailyPlayLimit <= 0 ||
		settings.EffectiveTicketUsageThreshold() <= 0 || settings.EffectiveDailyTicketLimit() <= 0 {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if !settings.GlobalEnabled {
		granted, grantErr := getZenxiangLiyuGrantForPlay(ctx, tx, cmd.UserID)
		if grantErr != nil {
			if errors.Is(grantErr, sql.ErrNoRows) {
				return nil, service.ErrZenxiangLiyuUnauthorized
			}
			return nil, grantErr
		}
		if !granted {
			return nil, service.ErrZenxiangLiyuUnauthorized
		}
	}
	prizes, err := listEnabledZenxiangLiyuPrizesForPlay(ctx, tx)
	if err != nil {
		return nil, err
	}
	prize, err := service.PickZenxiangLiyuPrize(prizes, cmd.Roll)
	if err != nil {
		return nil, err
	}

	var user struct {
		id      int64
		email   string
		role    string
		status  string
		balance float64
	}
	err = tx.QueryRowContext(ctx, `SELECT id, email, role, status, balance FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, cmd.UserID).
		Scan(&user.id, &user.email, &user.role, &user.status, &user.balance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if user.role != service.RoleUser || user.status != service.StatusActive {
		return nil, service.ErrZenxiangLiyuUnauthorized
	}

	todayUsageAmount, err := getZenxiangLiyuUserUsageAmountForPlay(ctx, tx, cmd.UserID, cmd.PlayDate)
	if err != nil {
		return nil, err
	}
	freePlayUsed, err := hasZenxiangLiyuUserFreePlayForPlay(ctx, tx, cmd.UserID, cmd.PlayDate)
	if err != nil {
		return nil, err
	}
	useFreePlay := service.IsZenxiangLiyuFreePlayAvailable(todayUsageAmount, freePlayUsed)
	effectiveTicketAmount := 0.0

	var playCount int
	playStart, playEnd := zenxiangLiyuUsageWindow(cmd.PlayDate)
	err = tx.QueryRowContext(ctx, `
		SELECT GREATEST(
			COUNT(*) - COALESCE((SELECT reset_count FROM zenxiang_liyu_daily_resets WHERE user_id = $1 AND play_date = $2), 0),
			0
		)
		FROM zenxiang_liyu_records WHERE user_id = $1 AND created_at >= $3 AND created_at < $4`, cmd.UserID, cmd.PlayDate, playStart, playEnd).Scan(&playCount)
	if err != nil {
		return nil, err
	}
	earnedTickets := service.CalculateZenxiangLiyuEarnedTickets(todayUsageAmount, settings)
	giftedTickets, err := getZenxiangLiyuGiftedTicketsForPlay(ctx, tx, cmd.UserID, cmd.PlayDate)
	if err != nil {
		return nil, err
	}
	earnedTickets += giftedTickets
	availableTickets, err := syncZenxiangLiyuTicketBalance(ctx, tx, cmd.UserID, cmd.PlayDate, settings)
	if err != nil {
		return nil, err
	}
	if availableTickets <= 0 {
		return nil, service.ErrZenxiangLiyuNoTicket
	}
	var consumedBatchID int64
	if err = tx.QueryRowContext(ctx, `
		WITH next_batch AS (
			SELECT id
			FROM zenxiang_liyu_ticket_batches
			WHERE user_id = $1 AND remaining_count > 0 AND expires_at > $2
			ORDER BY expires_at, id
			FOR UPDATE
			LIMIT 1
		)
		UPDATE zenxiang_liyu_ticket_batches batch
		SET remaining_count = remaining_count - 1, updated_at = NOW()
		FROM next_batch
		WHERE batch.id = next_batch.id
		RETURNING batch.id`, cmd.UserID, playStart).Scan(&consumedBatchID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrZenxiangLiyuNoTicket
		}
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE zenxiang_liyu_ticket_wallets
		SET balance = balance - 1, updated_at = NOW()
		WHERE user_id = $1 AND balance > 0`, cmd.UserID); err != nil {
		return nil, err
	}

	var balanceAfterTicket float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2 RETURNING balance`, effectiveTicketAmount, cmd.UserID).Scan(&balanceAfterTicket)
	if err != nil {
		return nil, err
	}

	configSnapshot, err := json.Marshal(map[string]any{
		"settings":                 settings,
		"prize":                    prize,
		"free_play":                useFreePlay,
		"today_usage_amount":       todayUsageAmount,
		"today_tickets_earned":     earnedTickets,
		"today_tickets_gifted":     giftedTickets,
		"today_tickets_used":       playCount,
		"tickets_available_before": availableTickets,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal zenxiang liyu config snapshot: %w", err)
	}
	userNetAmount := prize.RewardAmount - effectiveTicketAmount
	systemProfit := effectiveTicketAmount - prize.RewardAmount
	var recordID int64
	var playedAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO zenxiang_liyu_records (
			request_id, user_id, play_date, ticket_amount, reward_amount, user_net_amount,
			system_revenue, system_expense, system_profit, prize_id, prize_name_snapshot,
			probability_snapshot, config_snapshot, balance_before, balance_after_ticket, balance_after_reward
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at`,
		cmd.RequestID, cmd.UserID, cmd.PlayDate, effectiveTicketAmount, prize.RewardAmount, userNetAmount,
		effectiveTicketAmount, prize.RewardAmount, systemProfit, prize.ID, prize.Name, prize.Probability,
		configSnapshot, user.balance, balanceAfterTicket, balanceAfterTicket+prize.RewardAmount,
	).Scan(&recordID, &playedAt)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				return nil, fmt.Errorf("rollback zenxiang liyu play after request-id conflict: %w", rollbackErr)
			}
			return r.findZenxiangLiyuRecordAfterUniqueConflict(ctx, cmd.UserID, cmd.RequestID)
		}
		return nil, err
	}

	var balanceAfterReward float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2 RETURNING balance`, prize.RewardAmount, cmd.UserID).Scan(&balanceAfterReward)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuPlayResult{
		ID:                 recordID,
		Applied:            true,
		RequestID:          cmd.RequestID,
		PrizeID:            prize.ID,
		PrizeName:          prize.Name,
		RewardAmount:       prize.RewardAmount,
		TicketAmount:       effectiveTicketAmount,
		FreePlay:           useFreePlay,
		UserNetAmount:      userNetAmount,
		BalanceBefore:      user.balance,
		BalanceAfterTicket: balanceAfterTicket,
		BalanceAfterReward: balanceAfterReward,
		PlayedAt:           playedAt,
		LuckyCoinAvailable: settings.LuckyCoinEnabled && prize.RewardAmount > 0,
		LuckyCoinPlayed:    false,
	}, nil
}

func (r *zenxiangLiyuRepository) PlayLuckyCoin(ctx context.Context, cmd service.ZenxiangLiyuLuckyCoinCommand) (_ *service.ZenxiangLiyuLuckyCoinResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var settings struct {
		enabled     bool
		probability float64
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(lucky_coin_enabled, TRUE), COALESCE(lucky_coin_double_probability, 50)
		FROM zenxiang_liyu_settings WHERE id = 1 FOR SHARE`,
	).Scan(&settings.enabled, &settings.probability)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if err != nil {
		return nil, err
	}
	if !settings.enabled {
		return nil, service.ErrZenxiangLiyuLuckyCoinDisabled
	}

	var record struct {
		id           int64
		reward       float64
		played       bool
		balanceAfter sql.NullFloat64
	}
	err = tx.QueryRowContext(ctx, `
		SELECT id, reward_amount::double precision, COALESCE(lucky_coin_played, FALSE), balance_after_lucky
		FROM zenxiang_liyu_records
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, cmd.RecordID, cmd.UserID,
	).Scan(&record.id, &record.reward, &record.played, &record.balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuLuckyCoinUnavailable
	}
	if err != nil {
		return nil, err
	}
	if record.played {
		return nil, service.ErrZenxiangLiyuLuckyCoinAlreadyPlayed
	}
	if record.reward <= 0 {
		return nil, service.ErrZenxiangLiyuLuckyCoinUnavailable
	}

	outcome := "zero"
	adjustment := math.Round(-1.5*record.reward*zenxiangLiyuAmountScale) / zenxiangLiyuAmountScale
	if cmd.Roll < settings.probability {
		outcome = "double"
		adjustment = record.reward
	}

	var balanceAfter float64
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance`, adjustment, cmd.UserID,
	).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	var playedAt time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE zenxiang_liyu_records
		SET lucky_coin_played = TRUE,
		    lucky_coin_outcome = $1,
		    lucky_coin_adjustment = $2,
		    lucky_coin_played_at = NOW(),
		    balance_after_lucky = $3,
		    user_net_amount = user_net_amount + $2,
		    system_expense = system_expense + $2,
		    system_profit = system_profit - $2
		WHERE id = $4 AND user_id = $5 AND COALESCE(lucky_coin_played, FALSE) = FALSE
		RETURNING lucky_coin_played_at`,
		outcome, adjustment, balanceAfter, cmd.RecordID, cmd.UserID,
	).Scan(&playedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuLuckyCoinAlreadyPlayed
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuLuckyCoinResult{
		RecordID:           cmd.RecordID,
		Outcome:            outcome,
		OriginalReward:     record.reward,
		AdjustmentAmount:   adjustment,
		BalanceAfter:       balanceAfter,
		DoubleProbability:  settings.probability,
		PlayedAt:           playedAt,
		LuckyCoinAvailable: false,
	}, nil
}

func lockZenxiangLiyuSettingsForUpdate(ctx context.Context, tx *sql.Tx) error {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM zenxiang_liyu_settings WHERE id = 1 FOR UPDATE`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrZenxiangLiyuInvalidSettings
	}
	return err
}

func getZenxiangLiyuSettingsForPlay(ctx context.Context, tx *sql.Tx) (*service.ZenxiangLiyuSettings, error) {
	settings := &service.ZenxiangLiyuSettings{}
	err := tx.QueryRowContext(ctx, `
		SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit,
		       COALESCE(ticket_usage_threshold, 5), COALESCE(daily_ticket_limit, 3),
		       COALESCE(unit_sale_price, 0.1), COALESCE(unit_cost_price, 0.05),
		       COALESCE(lucky_coin_enabled, TRUE), COALESCE(lucky_coin_double_probability, 50)
		FROM zenxiang_liyu_settings WHERE id = 1 FOR UPDATE`,
	).Scan(
		&settings.GlobalEnabled, &settings.TicketAmount, &settings.MinimumBalance, &settings.DailyPlayLimit,
		&settings.TicketUsageThreshold, &settings.DailyTicketLimit, &settings.UnitSalePrice, &settings.UnitCostPrice,
		&settings.LuckyCoinEnabled, &settings.LuckyCoinProbability,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrZenxiangLiyuInvalidSettings
	}
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func getZenxiangLiyuGrantForPlay(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var granted bool
	err := tx.QueryRowContext(ctx, `
		SELECT enabled
		FROM zenxiang_liyu_user_grants
		WHERE user_id = $1
		FOR SHARE`, userID,
	).Scan(&granted)
	return granted, err
}

func getZenxiangLiyuUserUsageAmountForPlay(ctx context.Context, tx *sql.Tx, userID int64, playDate time.Time) (float64, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var amount float64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(actual_cost), 0)
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`, userID, start, end,
	).Scan(&amount)
	return amount, err
}

func hasZenxiangLiyuUserFreePlayForPlay(ctx context.Context, tx *sql.Tx, userID int64, playDate time.Time) (bool, error) {
	start, end := zenxiangLiyuUsageWindow(playDate)
	var used bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM zenxiang_liyu_records
			WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND ticket_amount = 0
		)`, userID, start, end,
	).Scan(&used)
	return used, err
}

func getZenxiangLiyuGiftedTicketsForPlay(ctx context.Context, tx *sql.Tx, userID int64, playDate time.Time) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(ticket_count), 0)
		FROM zenxiang_liyu_ticket_gifts
		WHERE user_id = $1 AND play_date = $2`, userID, playDate,
	).Scan(&count)
	return count, err
}

// syncZenxiangLiyuTicketBalance credits newly earned daily tickets once and keeps overflow from reappearing later.
func syncZenxiangLiyuTicketBalance(ctx context.Context, tx *sql.Tx, userID int64, throughDate time.Time, settings *service.ZenxiangLiyuSettings) (int, error) {
	if settings == nil || settings.EffectiveTicketUsageThreshold() <= 0 || settings.EffectiveDailyTicketLimit() <= 0 {
		return 0, service.ErrZenxiangLiyuInvalidSettings
	}
	balance, err := lockAndReconcileZenxiangLiyuTicketWallet(ctx, tx, userID, throughDate)
	if err != nil {
		return 0, err
	}

	_, throughEnd := zenxiangLiyuUsageWindow(throughDate)
	threshold := settings.EffectiveTicketUsageThreshold()
	dailyLimit := settings.EffectiveDailyTicketLimit()
	type dailyTicketCredit struct {
		usageDate time.Time
		count     int
	}
	creditRows, err := tx.QueryContext(ctx, `
		WITH earned AS (
			SELECT (created_at AT TIME ZONE 'Asia/Shanghai')::date AS usage_date,
			       LEAST(FLOOR(SUM(actual_cost) / $3)::integer, $4) AS ticket_count
			FROM usage_logs
			WHERE user_id = $1 AND created_at < $2
			  AND created_at >= (
				COALESCE(
					(SELECT ticket_carryover_started_on FROM zenxiang_liyu_settings WHERE id = 1),
					$5::date
				)::timestamp AT TIME ZONE 'Asia/Shanghai'
			  )
			  AND (
				(created_at AT TIME ZONE 'Asia/Shanghai')::date = $5::date
				OR NOT EXISTS (
					SELECT 1 FROM zenxiang_liyu_ticket_usage_credits credited
					WHERE credited.user_id = $1
					  AND credited.usage_date = (usage_logs.created_at AT TIME ZONE 'Asia/Shanghai')::date
				)
			  )
			GROUP BY (created_at AT TIME ZONE 'Asia/Shanghai')::date
		)
		SELECT earned.usage_date,
		       GREATEST(earned.ticket_count - COALESCE(credited.ticket_count, 0), 0) AS ticket_count
		FROM earned
		LEFT JOIN zenxiang_liyu_ticket_usage_credits credited
		  ON credited.user_id = $1 AND credited.usage_date = earned.usage_date
		WHERE earned.ticket_count > COALESCE(credited.ticket_count, 0)
		ORDER BY earned.usage_date`,
		userID, throughEnd, threshold, dailyLimit, throughDate,
	)
	if err != nil {
		return 0, err
	}
	credits := make([]dailyTicketCredit, 0)
	for creditRows.Next() {
		var credit dailyTicketCredit
		if err := creditRows.Scan(&credit.usageDate, &credit.count); err != nil {
			creditRows.Close()
			return 0, err
		}
		credits = append(credits, credit)
	}
	if err := creditRows.Close(); err != nil {
		return 0, err
	}
	if err := creditRows.Err(); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO zenxiang_liyu_ticket_usage_credits (user_id, usage_date, ticket_count)
		SELECT $1, (created_at AT TIME ZONE 'Asia/Shanghai')::date,
		       LEAST(FLOOR(SUM(actual_cost) / $3)::integer, $4)
		FROM usage_logs
		WHERE user_id = $1 AND created_at < $2
		  AND created_at >= (
			COALESCE(
				(SELECT ticket_carryover_started_on FROM zenxiang_liyu_settings WHERE id = 1),
				$5::date
			)::timestamp AT TIME ZONE 'Asia/Shanghai'
		  )
		  AND (
			(created_at AT TIME ZONE 'Asia/Shanghai')::date = $5::date
			OR NOT EXISTS (
				SELECT 1 FROM zenxiang_liyu_ticket_usage_credits credited
				WHERE credited.user_id = $1
				  AND credited.usage_date = (usage_logs.created_at AT TIME ZONE 'Asia/Shanghai')::date
			)
		  )
		GROUP BY (created_at AT TIME ZONE 'Asia/Shanghai')::date
		ON CONFLICT (user_id, usage_date)
		DO UPDATE SET ticket_count = GREATEST(
			zenxiang_liyu_ticket_usage_credits.ticket_count,
			EXCLUDED.ticket_count
		), updated_at = NOW()`, userID, throughEnd, threshold, dailyLimit, throughDate); err != nil {
		return 0, err
	}

	asOf, _ := zenxiangLiyuUsageWindow(throughDate)
	totalAccepted := 0
	for _, credit := range credits {
		expiresAt := zenxiangLiyuTicketExpiry(credit.usageDate)
		acceptedTickets := min(service.ZenxiangLiyuTicketCapacity-balance-totalAccepted, credit.count)
		if !expiresAt.After(asOf) || acceptedTickets <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO zenxiang_liyu_ticket_batches (
				user_id, source_type, source_key, granted_count, remaining_count, expires_at
			) VALUES ($1, 'usage', $2, $3, $3, $4)
			ON CONFLICT (user_id, source_type, source_key)
			DO UPDATE SET granted_count = zenxiang_liyu_ticket_batches.granted_count + EXCLUDED.granted_count,
			              remaining_count = zenxiang_liyu_ticket_batches.remaining_count + EXCLUDED.remaining_count,
			              updated_at = NOW()`,
			userID, credit.usageDate.Format("2006-01-02"), acceptedTickets, expiresAt,
		); err != nil {
			return 0, err
		}
		totalAccepted += acceptedTickets
	}
	if totalAccepted > 0 {
		if err := tx.QueryRowContext(ctx, `
			UPDATE zenxiang_liyu_ticket_wallets
			SET balance = LEAST($1, balance + $2), updated_at = NOW()
			WHERE user_id = $3
			RETURNING balance`, service.ZenxiangLiyuTicketCapacity, totalAccepted, userID,
		).Scan(&balance); err != nil {
			return 0, err
		}
	}
	return balance, nil
}

func lockAndReconcileZenxiangLiyuTicketWallet(ctx context.Context, tx *sql.Tx, userID int64, asOfDate time.Time) (int, error) {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO zenxiang_liyu_ticket_wallets (user_id, balance)
		VALUES ($1, 0)
		ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return 0, err
	}

	var balance int
	if err := tx.QueryRowContext(ctx, `
		SELECT balance
		FROM zenxiang_liyu_ticket_wallets
		WHERE user_id = $1
		FOR UPDATE`, userID).Scan(&balance); err != nil {
		return 0, err
	}

	asOf, _ := zenxiangLiyuUsageWindow(asOfDate)
	if _, err := tx.ExecContext(ctx, `
		UPDATE zenxiang_liyu_ticket_batches
		SET remaining_count = 0, updated_at = NOW()
		WHERE user_id = $1 AND remaining_count > 0 AND expires_at <= $2`, userID, asOf); err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `
		UPDATE zenxiang_liyu_ticket_wallets
		SET balance = LEAST($1, COALESCE((
			SELECT SUM(remaining_count)
			FROM zenxiang_liyu_ticket_batches
			WHERE user_id = $2 AND remaining_count > 0 AND expires_at > $3
		), 0)), updated_at = NOW()
		WHERE user_id = $2
		RETURNING balance`, service.ZenxiangLiyuTicketCapacity, userID, asOf).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func zenxiangLiyuTicketExpiry(playDate time.Time) time.Time {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(playDate.Year(), playDate.Month(), playDate.Day(), 0, 0, 0, 0, shanghai)
	return start.AddDate(0, 0, service.ZenxiangLiyuTicketRetentionDays).UTC()
}

func zenxiangLiyuUsageWindow(playDate time.Time) (time.Time, time.Time) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(playDate.Year(), playDate.Month(), playDate.Day(), 0, 0, 0, 0, shanghai)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func listEnabledZenxiangLiyuPrizesForPlay(ctx context.Context, tx *sql.Tx) ([]service.ZenxiangLiyuPrize, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, reward_amount, probability, enabled, sort_order
		FROM zenxiang_liyu_prizes
		WHERE enabled = TRUE
		ORDER BY sort_order, id
		FOR SHARE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanZenxiangLiyuPrizes(rows)
}

// disableOmittedZenxiangLiyuPrizes preserves referenced prize history while removing
// omitted prizes from the active draw configuration.
func disableOmittedZenxiangLiyuPrizes(ctx context.Context, tx *sql.Tx, saved []service.ZenxiangLiyuPrize) error {
	ids := make([]int64, 0, len(saved))
	for _, prize := range saved {
		ids = append(ids, prize.ID)
	}

	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx, `UPDATE zenxiang_liyu_prizes SET enabled = FALSE, updated_at = NOW() WHERE enabled = TRUE`)
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE zenxiang_liyu_prizes
		SET enabled = FALSE, updated_at = NOW()
		WHERE enabled = TRUE AND NOT (id = ANY($1))`, pq.Array(ids))
	return err
}

func (r *zenxiangLiyuRepository) findZenxiangLiyuRecordAfterUniqueConflict(ctx context.Context, userID int64, requestID string) (_ *service.ZenxiangLiyuPlayResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	record, err := findZenxiangLiyuRecord(ctx, tx, userID, requestID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func scanZenxiangLiyuPrizes(rows *sql.Rows) ([]service.ZenxiangLiyuPrize, error) {
	prizes := make([]service.ZenxiangLiyuPrize, 0)
	for rows.Next() {
		var prize service.ZenxiangLiyuPrize
		if err := rows.Scan(&prize.ID, &prize.Name, &prize.RewardAmount, &prize.Probability, &prize.Enabled, &prize.SortOrder); err != nil {
			return nil, err
		}
		prizes = append(prizes, prize)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return prizes, nil
}

func findZenxiangLiyuRecord(ctx context.Context, tx *sql.Tx, userID int64, requestID string) (*service.ZenxiangLiyuPlayResult, error) {
	result := &service.ZenxiangLiyuPlayResult{Applied: false}
	var prizeID sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT id, request_id, user_id, ticket_amount, reward_amount, user_net_amount,
			system_revenue, system_expense, system_profit, prize_id, prize_name_snapshot,
			probability_snapshot, balance_before, balance_after_ticket, balance_after_reward, created_at,
			COALESCE(lucky_coin_played, FALSE)
		FROM zenxiang_liyu_records WHERE user_id = $1 AND request_id = $2`, userID, requestID,
	).Scan(
		&result.ID, &result.RequestID, new(int64), &result.TicketAmount, &result.RewardAmount, &result.UserNetAmount,
		new(float64), new(float64), new(float64), &prizeID, &result.PrizeName,
		new(float64), &result.BalanceBefore, &result.BalanceAfterTicket, &result.BalanceAfterReward, &result.PlayedAt,
		&result.LuckyCoinPlayed,
	)
	if err != nil {
		return nil, err
	}
	if prizeID.Valid {
		result.PrizeID = prizeID.Int64
	}
	result.FreePlay = result.TicketAmount == 0
	result.LuckyCoinAvailable = result.RewardAmount > 0 && !result.LuckyCoinPlayed
	return result, nil
}
