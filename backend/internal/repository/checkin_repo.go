package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type checkinRepository struct {
	db *sql.DB
}

func NewCheckinRepository(db *sql.DB) service.CheckinRepository {
	return &checkinRepository{db: db}
}

func (r *checkinRepository) HasCheckedInOnDate(ctx context.Context, userID int64, date string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM checkin_records WHERE user_id = $1 AND checkin_date = $2::date
		)
	`, userID, date).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *checkinRepository) CreateAndCredit(ctx context.Context, record *service.CheckinRecord) (*service.CheckinRecord, error) {
	if record == nil {
		return nil, errors.New("checkin record is required")
	}
	if record.BaseRewardPoints <= 0 {
		record.BaseRewardPoints = record.RewardPoints
	}
	if strings.TrimSpace(record.BonusStatus) == "" {
		record.BonusStatus = service.CheckinBonusStatusNone
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO checkin_records (
			user_id, checkin_date, reward_points, base_reward_points, bonus_status, bonus_delta_points, user_timezone
		)
		VALUES ($1, $2::date, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`, record.UserID, record.CheckinDate, record.RewardPoints, record.BaseRewardPoints, record.BonusStatus, record.BonusDeltaPoints, record.UserTimezone).Scan(&record.ID, &createdAt)
	if err != nil {
		if isCheckinUniqueViolation(err) {
			return nil, service.ErrCheckinAlreadyToday
		}
		return nil, err
	}
	record.CreatedAt = createdAt

	var pointsBefore int64
	if err := tx.QueryRowContext(ctx, `
		SELECT points
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, record.UserID).Scan(&pointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	pointsAfter := pointsBefore + record.RewardPoints

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET points = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, record.UserID, pointsAfter); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_game_key, reason
		) VALUES ($1, 'checkin_reward', $2, $3, $4, 'checkin', 'daily checkin reward')
	`, record.UserID, record.RewardPoints, pointsBefore, pointsAfter); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return record, nil
}

func (r *checkinRepository) ListByUserAndDateRange(ctx context.Context, userID int64, startDate, endDate string) ([]service.CheckinRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, checkin_date::text, reward_points, base_reward_points, bonus_status, bonus_delta_points, user_timezone, created_at, bonus_played_at
		FROM checkin_records
		WHERE user_id = $1 AND checkin_date >= $2::date AND checkin_date <= $3::date
		ORDER BY checkin_date DESC
	`, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]service.CheckinRecord, 0)
	for rows.Next() {
		record, err := scanCheckinRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *checkinRepository) GetByUserAndDate(ctx context.Context, userID int64, date string) (*service.CheckinRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, checkin_date::text, reward_points, base_reward_points, bonus_status, bonus_delta_points, user_timezone, created_at, bonus_played_at
		FROM checkin_records
		WHERE user_id = $1 AND checkin_date = $2::date
		LIMIT 1
	`, userID, date)

	record, err := scanCheckinRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *checkinRepository) ApplyBonusOutcome(ctx context.Context, userID int64, date, outcome string, deltaPoints int64) (*service.CheckinRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		UPDATE checkin_records
		SET reward_points = reward_points + $1,
		    bonus_status = $2,
		    bonus_delta_points = $1,
		    bonus_played_at = NOW()
		WHERE user_id = $3
		  AND checkin_date = $4::date
		  AND bonus_status = $5
		RETURNING id, user_id, checkin_date::text, reward_points, base_reward_points, bonus_status, bonus_delta_points, user_timezone, created_at, bonus_played_at
	`, deltaPoints, outcome, userID, date, service.CheckinBonusStatusNone)

	record, err := scanCheckinRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCheckinLuckyBonusAlreadyPlayed
		}
		return nil, err
	}

	var pointsBefore int64
	if err := tx.QueryRowContext(ctx, `
		SELECT points
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, userID).Scan(&pointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	pointsAfter := pointsBefore + deltaPoints
	if pointsAfter < 0 {
		return nil, service.ErrGameCenterInsufficientPoints
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET points = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, pointsAfter); err != nil {
		return nil, err
	}

	entryType := "checkin_bonus_lose"
	reason := "checkin lucky bonus lose"
	if deltaPoints > 0 {
		entryType = "checkin_bonus_win"
		reason = "checkin lucky bonus win"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_game_key, reason
		) VALUES ($1, $2, $3, $4, $5, 'checkin', $6)
	`, userID, entryType, deltaPoints, pointsBefore, pointsAfter, reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &record, nil
}

func (r *checkinRepository) GetUserTotals(ctx context.Context, userID int64) (int64, int64, error) {
	var totalCount int64
	var totalClaimedPoints int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(reward_points), 0)
		FROM checkin_records
		WHERE user_id = $1
	`, userID).Scan(&totalCount, &totalClaimedPoints)
	if err != nil {
		return 0, 0, err
	}
	return totalCount, totalClaimedPoints, nil
}

func (r *checkinRepository) ListAdminRecords(ctx context.Context, page, pageSize int, search, date, timezone, sortBy, sortOrder string) ([]service.AdminCheckinRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	whereParts := []string{"1=1"}
	args := make([]any, 0)
	argIndex := 1

	search = strings.TrimSpace(search)
	if search != "" {
		whereParts = append(whereParts, fmt.Sprintf("(u.email ILIKE $%d OR u.username ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}

	date = strings.TrimSpace(date)
	if date != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date = $%d::date", argIndex))
		args = append(args, date)
		argIndex++
	}

	timezone = strings.TrimSpace(timezone)
	if timezone != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.user_timezone = $%d", argIndex))
		args = append(args, timezone)
		argIndex++
	}

	whereClause := strings.Join(whereParts, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM checkin_records c JOIN users u ON u.id = c.user_id WHERE %s`, whereClause)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderClause := adminCheckinOrderBy(sortBy, sortOrder)
	args = append(args, pageSize, (page-1)*pageSize)
	dataQuery := fmt.Sprintf(`
		SELECT c.id, c.user_id, u.email, u.username, c.checkin_date::text, c.reward_points, c.user_timezone, c.created_at
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderClause, argIndex, argIndex+1)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]service.AdminCheckinRecord, 0)
	for rows.Next() {
		record, err := scanAdminCheckinRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *checkinRepository) GetAdminOverview(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) (service.AdminCheckinOverview, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*),
			COALESCE(SUM(c.reward_points), 0),
			COUNT(*) FILTER (WHERE c.checkin_date = CURRENT_DATE),
			COALESCE(AVG(c.reward_points), 0)
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
	`, whereClause)

	var overview service.AdminCheckinOverview
	var totalRewardPoints int64
	var avgRewardPoints float64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&overview.TotalCheckins,
		&totalRewardPoints,
		&overview.TodayCheckins,
		&avgRewardPoints,
	)
	if err != nil {
		return service.AdminCheckinOverview{}, err
	}
	overview.TotalRewardPoints = totalRewardPoints
	overview.AvgRewardPoints = int64(math.Round(avgRewardPoints))
	return overview, nil
}

func (r *checkinRepository) GetAdminTrend(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTrendPoint, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.checkin_date::text, COUNT(*), COALESCE(SUM(c.reward_points), 0)
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		GROUP BY c.checkin_date
		ORDER BY c.checkin_date ASC
	`, whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]service.AdminCheckinTrendPoint, 0)
	for rows.Next() {
		var point service.AdminCheckinTrendPoint
		var claimedPoints int64
		if err := rows.Scan(&point.Date, &point.CheckinCount, &claimedPoints); err != nil {
			return nil, err
		}
		point.RewardPoints = claimedPoints
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (r *checkinRepository) GetAdminRewardDistribution(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinRewardBucket, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.reward_points
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		ORDER BY c.reward_points ASC
	`, whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rewardPointsSeries := make([]int64, 0)
	for rows.Next() {
		var claimedPoints int64
		if err := rows.Scan(&claimedPoints); err != nil {
			return nil, err
		}
		rewardPointsSeries = append(rewardPointsSeries, claimedPoints)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(rewardPointsSeries) == 0 {
		return []service.AdminCheckinRewardBucket{}, nil
	}

	buckets := make(map[string]int64)
	for _, claimedPoints := range rewardPointsSeries {
		key := fmt.Sprintf("%d", claimedPoints)
		buckets[key]++
	}

	result := make([]service.AdminCheckinRewardBucket, 0, len(buckets))
	for label, count := range buckets {
		value, _ := strconv.ParseInt(label, 10, 64)
		result = append(result, service.AdminCheckinRewardBucket{
			Label:        label,
			Min:          value,
			Max:          value,
			Count:        count,
			RewardPoints: value * count,
		})
	}
	return result, nil
}

func (r *checkinRepository) GetAdminTopUsers(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTopUser, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			c.user_id,
			COALESCE(u.email, ''),
			COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', c.user_id)),
			COUNT(*) AS total_checkins,
			COALESCE(SUM(c.reward_points), 0) AS total_reward_points
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		GROUP BY c.user_id, u.email, u.username
		ORDER BY total_reward_points DESC, total_checkins DESC, c.user_id ASC
		LIMIT 10
	`, whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.AdminCheckinTopUser, 0)
	for rows.Next() {
		var item service.AdminCheckinTopUser
		var claimedPoints int64
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.TotalCheckins, &claimedPoints); err != nil {
			return nil, err
		}
		item.TotalRewardPoints = claimedPoints
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanCheckinRecord(row interface{ Scan(dest ...any) error }) (service.CheckinRecord, error) {
	var record service.CheckinRecord
	var claimedPoints int64
	var basePoints int64
	var bonusPointsDelta int64
	var bonusPlayedAt sql.NullTime
	err := row.Scan(
		&record.ID,
		&record.UserID,
		&record.CheckinDate,
		&claimedPoints,
		&basePoints,
		&record.BonusStatus,
		&bonusPointsDelta,
		&record.UserTimezone,
		&record.CreatedAt,
		&bonusPlayedAt,
	)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	record.RewardPoints = claimedPoints
	record.BaseRewardPoints = basePoints
	record.BonusDeltaPoints = bonusPointsDelta
	if bonusPlayedAt.Valid {
		record.BonusPlayedAt = &bonusPlayedAt.Time
	}
	return record, nil
}

func isCheckinUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func scanAdminCheckinRecord(row interface{ Scan(dest ...any) error }) (service.AdminCheckinRecord, error) {
	var record service.AdminCheckinRecord
	err := row.Scan(
		&record.ID,
		&record.UserID,
		&record.Email,
		&record.Username,
		&record.CheckinDate,
		&record.RewardPoints,
		&record.UserTimezone,
		&record.CreatedAt,
	)
	if err != nil {
		return service.AdminCheckinRecord{}, err
	}
	return record, nil
}

func adminCheckinOrderBy(sortBy, sortOrder string) string {
	field := "c.created_at"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "reward_points":
		field = "c.reward_points"
	case "checkin_date":
		field = "c.checkin_date"
	case "email":
		field = "u.email"
	case "username":
		field = "u.username"
	case "created_at", "":
		field = "c.created_at"
	}

	order := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		order = "ASC"
	}
	return fmt.Sprintf("%s %s, c.id %s", field, order, order)
}

func buildAdminCheckinAnalyticsWhere(filter service.AdminCheckinAnalyticsFilter) (string, []any) {
	whereParts := []string{"1=1"}
	args := make([]any, 0, 4)
	argIndex := 1

	if startDate := strings.TrimSpace(filter.StartDate); startDate != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date >= $%d::date", argIndex))
		args = append(args, startDate)
		argIndex++
	}
	if endDate := strings.TrimSpace(filter.EndDate); endDate != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date <= $%d::date", argIndex))
		args = append(args, endDate)
		argIndex++
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		whereParts = append(whereParts, fmt.Sprintf("(u.email ILIKE $%d OR u.username ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if timezone := strings.TrimSpace(filter.Timezone); timezone != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.user_timezone = $%d", argIndex))
		args = append(args, timezone)
	}

	return strings.Join(whereParts, " AND "), args
}
