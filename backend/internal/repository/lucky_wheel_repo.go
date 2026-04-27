package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type luckyWheelRepository struct {
	db *sql.DB
}

func NewLuckyWheelRepository(_ *dbent.Client, sqlDB *sql.DB) service.LuckyWheelRepository {
	return &luckyWheelRepository{db: sqlDB}
}

func (r *luckyWheelRepository) GetUserAssets(ctx context.Context, userID int64) (*service.GameCenterAssets, error) {
	var assets service.GameCenterAssets
	err := r.db.QueryRowContext(ctx, `SELECT balance, points FROM users WHERE id = $1`, userID).Scan(&assets.Balance, &assets.Points)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	return &assets, err
}

func (r *luckyWheelRepository) CountUserSpinsOnDate(ctx context.Context, userID int64, spinDate string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_lucky_wheel_spins
		WHERE user_id = $1 AND spin_date = $2
	`, userID, spinDate).Scan(&count)
	return count, err
}

func (r *luckyWheelRepository) ApplySpin(ctx context.Context, input service.LuckyWheelApplySpinInput) (*service.LuckyWheelSpinRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	record := &service.LuckyWheelSpinRecord{
		UserID:      input.UserID,
		SpinDate:    input.SpinDate,
		PrizeKey:    input.Prize.Key,
		PrizeLabel:  input.Prize.Label,
		PrizeType:   input.Prize.Type,
		DeltaPoints: input.Prize.DeltaPoints,
		Probability: input.Prize.Probability,
	}

	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(email, ''), COALESCE(NULLIF(username, ''), email, CONCAT('user-', id)), points
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, input.UserID).Scan(&record.Email, &record.Username, &record.PointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	var used int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_lucky_wheel_spins
		WHERE user_id = $1 AND spin_date = $2
	`, input.UserID, input.SpinDate).Scan(&used); err != nil {
		return nil, err
	}
	if used >= input.DailyLimit {
		return nil, service.ErrLuckyWheelDailyLimitReached
	}

	record.SpinIndex = used + 1
	record.PointsAfter = record.PointsBefore + record.DeltaPoints
	if record.PointsAfter < 0 {
		return nil, service.ErrLuckyWheelInsufficientPoints
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET points = $2, updated_at = NOW()
		WHERE id = $1
	`, input.UserID, record.PointsAfter); err != nil {
		return nil, err
	}

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO game_lucky_wheel_spins (
			user_id, spin_date, spin_index, prize_key, prize_label, prize_type, delta_points, points_before, points_after, probability, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING id, created_at
	`, input.UserID, input.SpinDate, record.SpinIndex, record.PrizeKey, record.PrizeLabel, record.PrizeType, record.DeltaPoints, record.PointsBefore, record.PointsAfter, record.Probability, input.TriggeredAt).Scan(&record.ID, &record.CreatedAt); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_game_key, reason
		) VALUES ($1, 'lucky_wheel_spin', $2, $3, $4, $5, $6)
	`, input.UserID, record.DeltaPoints, record.PointsBefore, record.PointsAfter, service.LuckyWheelGameKey, record.PrizeLabel); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *luckyWheelRepository) ListUserSpins(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	filter := service.LuckyWheelAdminSpinFilter{UserID: &userID}
	return r.listSpins(ctx, params, filter, false)
}

func (r *luckyWheelRepository) ListAdminSpins(ctx context.Context, params pagination.PaginationParams, filter service.LuckyWheelAdminSpinFilter) ([]service.LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	return r.listSpins(ctx, params, filter, true)
}

func (r *luckyWheelRepository) ListLeaderboard(ctx context.Context, spinDate string, limit int) ([]service.LuckyWheelLeaderboardItem, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			ranked.user_id,
			ranked.email,
			ranked.username,
			ranked.points,
			ranked.net_points,
			ranked.spin_count,
			ranked.best_delta,
			ranked.best_prize_label
		FROM (
			SELECT
				s.user_id,
				COALESCE(u.email, '') AS email,
				COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', s.user_id)) AS username,
				u.points,
				COALESCE(SUM(s.delta_points), 0) AS net_points,
				COUNT(*)::bigint AS spin_count,
				COALESCE(MAX(s.delta_points), 0) AS best_delta,
				(ARRAY_AGG(s.prize_label ORDER BY s.delta_points DESC, s.id DESC))[1] AS best_prize_label
			FROM game_lucky_wheel_spins s
			JOIN users u ON u.id = s.user_id
			WHERE s.spin_date = $1
			GROUP BY s.user_id, u.email, u.username, u.points
			ORDER BY net_points DESC, spin_count DESC, s.user_id ASC
			LIMIT $2
		) ranked
	`, spinDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.LuckyWheelLeaderboardItem, 0, limit)
	rank := 1
	for rows.Next() {
		var item service.LuckyWheelLeaderboardItem
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.Points, &item.NetPoints, &item.SpinCount, &item.BestDelta, &item.BestPrizeLabel); err != nil {
			return nil, err
		}
		item.Rank = rank
		rank++
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *luckyWheelRepository) listSpins(ctx context.Context, params pagination.PaginationParams, filter service.LuckyWheelAdminSpinFilter, includeUserMeta bool) ([]service.LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	where, args := buildLuckyWheelWhere("s", filter)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_lucky_wheel_spins s`+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			s.id,
			s.user_id,
			COALESCE(u.email, ''),
			COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', s.user_id)),
			s.spin_date::text,
			s.spin_index,
			s.prize_key,
			s.prize_label,
			s.prize_type,
			s.delta_points,
			s.points_before,
			s.points_after,
			s.probability,
			s.created_at
		FROM game_lucky_wheel_spins s
		LEFT JOIN users u ON u.id = s.user_id`+where+`
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $`+argPos(len(args)-1)+` OFFSET $`+argPos(len(args))+`
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.LuckyWheelSpinRecord, 0)
	for rows.Next() {
		var item service.LuckyWheelSpinRecord
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.SpinDate,
			&item.SpinIndex,
			&item.PrizeKey,
			&item.PrizeLabel,
			&item.PrizeType,
			&item.DeltaPoints,
			&item.PointsBefore,
			&item.PointsAfter,
			&item.Probability,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		if !includeUserMeta {
			item.Email = ""
		}
		items = append(items, item)
	}
	return items, buildPageResult(total, params), rows.Err()
}

func buildLuckyWheelWhere(alias string, filter service.LuckyWheelAdminSpinFilter) (string, []any) {
	column := func(name string) string {
		if strings.TrimSpace(alias) == "" {
			return name
		}
		return fmt.Sprintf("%s.%s", alias, name)
	}
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column("user_id"), len(args)))
	}
	if filter.StartTime != nil {
		args = append(args, *filter.StartTime)
		conditions = append(conditions, fmt.Sprintf("%s >= $%d", column("created_at"), len(args)))
	}
	if filter.EndTime != nil {
		args = append(args, *filter.EndTime)
		conditions = append(conditions, fmt.Sprintf("%s <= $%d", column("created_at"), len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
