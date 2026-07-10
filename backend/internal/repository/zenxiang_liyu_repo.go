package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type zenxiangLiyuRepository struct {
	client *ent.Client
	db     *sql.DB
}

func NewZenxiangLiyuRepository(client *ent.Client, sqlDB *sql.DB) service.ZenxiangLiyuRepository {
	return &zenxiangLiyuRepository{client: client, db: sqlDB}
}

func (r *zenxiangLiyuRepository) GetSettings(ctx context.Context) (*service.ZenxiangLiyuSettings, error) {
	settings := &service.ZenxiangLiyuSettings{}
	err := r.db.QueryRowContext(ctx, `
		SELECT global_enabled, ticket_amount, minimum_balance, daily_play_limit
		FROM zenxiang_liyu_settings WHERE id = 1`,
	).Scan(&settings.GlobalEnabled, &settings.TicketAmount, &settings.MinimumBalance, &settings.DailyPlayLimit)
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
		SET global_enabled = $1, ticket_amount = $2, minimum_balance = $3, daily_play_limit = $4, updated_at = NOW()
		WHERE id = 1
		RETURNING global_enabled, ticket_amount, minimum_balance, daily_play_limit`,
		settings.GlobalEnabled, settings.TicketAmount, settings.MinimumBalance, settings.DailyPlayLimit,
	).Scan(&updated.GlobalEnabled, &updated.TicketAmount, &updated.MinimumBalance, &updated.DailyPlayLimit)
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
	if prize.ID == 0 {
		created := &service.ZenxiangLiyuPrize{}
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO zenxiang_liyu_prizes (name, reward_amount, probability, enabled, sort_order)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, reward_amount, probability, enabled, sort_order`,
			prize.Name, prize.RewardAmount, prize.Probability, prize.Enabled, prize.SortOrder,
		).Scan(&created.ID, &created.Name, &created.RewardAmount, &created.Probability, &created.Enabled, &created.SortOrder)
		if err != nil {
			return nil, err
		}
		return created, nil
	}

	updated := &service.ZenxiangLiyuPrize{}
	err := r.db.QueryRowContext(ctx, `
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

func (r *zenxiangLiyuRepository) DeletePrize(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM zenxiang_liyu_prizes WHERE id = $1`, id)
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

func (r *zenxiangLiyuRepository) IsUserGranted(ctx context.Context, userID int64) (bool, error) {
	var granted bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM zenxiang_liyu_user_grants WHERE user_id = $1 AND enabled = TRUE)`, userID,
	).Scan(&granted)
	return granted, err
}

func (r *zenxiangLiyuRepository) CountUserPlaysOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM zenxiang_liyu_records WHERE user_id = $1 AND play_date = $2`, userID, playDate,
	).Scan(&count)
	return count, err
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

	if existing, lookupErr := findZenxiangLiyuRecord(ctx, tx, cmd.RequestID); lookupErr == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
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
	if user.balance <= cmd.Settings.MinimumBalance || user.balance < cmd.Settings.TicketAmount {
		return nil, service.ErrZenxiangLiyuInsufficientBalance
	}

	var playCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM zenxiang_liyu_records WHERE user_id = $1 AND play_date = $2`, cmd.UserID, cmd.PlayDate).Scan(&playCount)
	if err != nil {
		return nil, err
	}
	if playCount >= cmd.Settings.DailyPlayLimit {
		return nil, service.ErrZenxiangLiyuDailyLimitReached
	}

	var balanceAfterTicket float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2 RETURNING balance`, cmd.Settings.TicketAmount, cmd.UserID).Scan(&balanceAfterTicket)
	if err != nil {
		return nil, err
	}

	configSnapshot, err := json.Marshal(cmd.ConfigSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal zenxiang liyu config snapshot: %w", err)
	}
	userNetAmount := cmd.Prize.RewardAmount - cmd.Settings.TicketAmount
	systemProfit := cmd.Settings.TicketAmount - cmd.Prize.RewardAmount
	var playedAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO zenxiang_liyu_records (
			request_id, user_id, play_date, ticket_amount, reward_amount, user_net_amount,
			system_revenue, system_expense, system_profit, prize_id, prize_name_snapshot,
			probability_snapshot, config_snapshot, balance_before, balance_after_ticket, balance_after_reward
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at`,
		cmd.RequestID, cmd.UserID, cmd.PlayDate, cmd.Settings.TicketAmount, cmd.Prize.RewardAmount, userNetAmount,
		cmd.Settings.TicketAmount, cmd.Prize.RewardAmount, systemProfit, cmd.Prize.ID, cmd.Prize.Name, cmd.Prize.Probability,
		configSnapshot, user.balance, balanceAfterTicket, balanceAfterTicket+cmd.Prize.RewardAmount,
	).Scan(&playedAt)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				return nil, fmt.Errorf("rollback zenxiang liyu play after request-id conflict: %w", rollbackErr)
			}
			return r.findZenxiangLiyuRecordAfterUniqueConflict(ctx, cmd.RequestID)
		}
		return nil, err
	}

	var balanceAfterReward float64
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance = balance + $1, total_recharged = total_recharged + $1, updated_at = NOW() WHERE id = $2 RETURNING balance`, cmd.Prize.RewardAmount, cmd.UserID).Scan(&balanceAfterReward)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuPlayResult{
		Applied:            true,
		RequestID:          cmd.RequestID,
		PrizeID:            cmd.Prize.ID,
		PrizeName:          cmd.Prize.Name,
		RewardAmount:       cmd.Prize.RewardAmount,
		TicketAmount:       cmd.Settings.TicketAmount,
		UserNetAmount:      userNetAmount,
		BalanceBefore:      user.balance,
		BalanceAfterTicket: balanceAfterTicket,
		BalanceAfterReward: balanceAfterReward,
		PlayedAt:           playedAt,
	}, nil
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

func (r *zenxiangLiyuRepository) findZenxiangLiyuRecordAfterUniqueConflict(ctx context.Context, requestID string) (_ *service.ZenxiangLiyuPlayResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	record, err := findZenxiangLiyuRecord(ctx, tx, requestID)
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

func findZenxiangLiyuRecord(ctx context.Context, tx *sql.Tx, requestID string) (*service.ZenxiangLiyuPlayResult, error) {
	result := &service.ZenxiangLiyuPlayResult{Applied: false}
	var prizeID sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT id, request_id, user_id, ticket_amount, reward_amount, user_net_amount,
			system_revenue, system_expense, system_profit, prize_id, prize_name_snapshot,
			probability_snapshot, balance_before, balance_after_ticket, balance_after_reward, created_at
		FROM zenxiang_liyu_records WHERE request_id = $1`, requestID,
	).Scan(
		new(int64), &result.RequestID, new(int64), &result.TicketAmount, &result.RewardAmount, &result.UserNetAmount,
		new(float64), new(float64), new(float64), &prizeID, &result.PrizeName,
		new(float64), &result.BalanceBefore, &result.BalanceAfterTicket, &result.BalanceAfterReward, &result.PlayedAt,
	)
	if err != nil {
		return nil, err
	}
	if prizeID.Valid {
		result.PrizeID = prizeID.Int64
	}
	return result, nil
}
