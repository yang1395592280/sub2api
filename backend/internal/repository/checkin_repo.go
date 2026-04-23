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
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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
	err := r.db.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1 FROM checkin_records WHERE user_id = $1 AND checkin_date = $2
		)`,
		userID,
		date,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *checkinRepository) CreateAndCredit(ctx context.Context, record *service.CheckinRecord) (*service.CheckinRecord, error) {
	if record == nil {
		return nil, errors.New("checkin record is required")
	}
	if record.BaseRewardAmount <= 0 {
		record.BaseRewardAmount = record.RewardAmount
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
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO checkin_records (
			user_id, checkin_date, reward_amount, base_reward_amount, bonus_status, bonus_delta_amount, user_timezone
		)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		record.UserID,
		record.CheckinDate,
		record.RewardAmount,
		record.BaseRewardAmount,
		record.BonusStatus,
		record.BonusDeltaAmount,
		record.UserTimezone,
	).Scan(&record.ID, &createdAt)
	if err != nil {
		if isCheckinUniqueViolation(err) {
			return nil, service.ErrCheckinAlreadyToday
		}
		return nil, err
	}
	record.CreatedAt = createdAt

	result, err := tx.ExecContext(
		ctx,
		`UPDATE users
		 SET balance = balance + $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL`,
		record.RewardAmount,
		record.UserID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrUserNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return record, nil
}

func (r *checkinRepository) ListByUserAndDateRange(ctx context.Context, userID int64, startDate, endDate string) ([]service.CheckinRecord, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, checkin_date, reward_amount, base_reward_amount, bonus_status, bonus_delta_amount, user_timezone, created_at, bonus_played_at
		 FROM checkin_records
		 WHERE user_id = $1 AND checkin_date >= $2 AND checkin_date <= $3
		 ORDER BY checkin_date DESC`,
		userID,
		startDate,
		endDate,
	)
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

func (r *checkinRepository) ListTimelineItemsByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filter string) ([]service.UserActivityTimelineItem, *pagination.PaginationResult, error) {
	args := []any{userID, service.CheckinBonusStatusNone}
	whereClause := "1=1"
	if strings.TrimSpace(filter) == "checkin_reward" || strings.TrimSpace(filter) == "checkin_bonus" {
		args = append(args, strings.TrimSpace(filter))
		whereClause = fmt.Sprintf("event_type = $%d", len(args))
	}

	queryPrefix := `
		WITH timeline AS (
			SELECT
				c.id AS source_id,
				'checkin_reward'::text AS event_type,
				c.created_at AS event_at,
				c.reward_amount::text AS value,
				c.checkin_date,
				COALESCE(c.user_timezone, '') AS user_timezone,
				COALESCE(c.bonus_status, '') AS bonus_status,
				c.base_reward_amount::text AS base_reward_amount,
				c.reward_amount::text AS reward_amount,
				0 AS event_order
			FROM checkin_records c
			WHERE c.user_id = $1
			UNION ALL
			SELECT
				c.id AS source_id,
				'checkin_bonus'::text AS event_type,
				c.bonus_played_at AS event_at,
				c.bonus_delta_amount::text AS value,
				c.checkin_date,
				COALESCE(c.user_timezone, '') AS user_timezone,
				COALESCE(c.bonus_status, '') AS bonus_status,
				c.base_reward_amount::text AS base_reward_amount,
				c.reward_amount::text AS reward_amount,
				1 AS event_order
			FROM checkin_records c
			WHERE c.user_id = $1
				AND c.bonus_played_at IS NOT NULL
				AND (COALESCE(c.bonus_status, '') <> $2 OR c.bonus_delta_amount <> 0)
		)
	`

	countQuery := queryPrefix + fmt.Sprintf(`SELECT COUNT(*) FROM timeline WHERE %s`, whereClause)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	dataArgs := append(append([]any(nil), args...), params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, queryPrefix+fmt.Sprintf(`
		SELECT source_id, event_type, event_at, value, checkin_date, user_timezone, bonus_status, base_reward_amount, reward_amount
		FROM timeline
		WHERE %s
		ORDER BY event_at DESC, source_id DESC, event_order ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(dataArgs)-1, len(dataArgs)), dataArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.UserActivityTimelineItem, 0)
	for rows.Next() {
		var (
			sourceID         int64
			eventType        string
			eventAt          time.Time
			valueRaw         string
			checkinDate      string
			userTimezone     string
			bonusStatus      string
			baseRewardRaw    string
			rewardAmountRaw  string
		)
		if err := rows.Scan(&sourceID, &eventType, &eventAt, &valueRaw, &checkinDate, &userTimezone, &bonusStatus, &baseRewardRaw, &rewardAmountRaw); err != nil {
			return nil, nil, err
		}

		value, err := strconv.ParseFloat(valueRaw, 64)
		if err != nil {
			return nil, nil, err
		}
		item := service.UserActivityTimelineItem{
			Type:      eventType,
			Value:     value,
			CreatedAt: eventAt,
			Details: map[string]any{
				"checkin_date":  checkinDate,
				"user_timezone": userTimezone,
			},
		}

		switch eventType {
		case "checkin_reward":
			item.ID = fmt.Sprintf("checkin-%d", sourceID)
		case "checkin_bonus":
			baseRewardAmount, err := strconv.ParseFloat(baseRewardRaw, 64)
			if err != nil {
				return nil, nil, err
			}
			rewardAmount, err := strconv.ParseFloat(rewardAmountRaw, 64)
			if err != nil {
				return nil, nil, err
			}
			if strings.TrimSpace(bonusStatus) == "" {
				bonusStatus = service.CheckinBonusStatusNone
			}
			item.ID = fmt.Sprintf("checkin-bonus-%d", sourceID)
			item.Details["bonus_status"] = bonusStatus
			item.Details["base_reward_amount"] = baseRewardAmount
			item.Details["reward_amount"] = rewardAmount
		}

		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, paginationResultFromTotal(total, params), nil
}

func (r *checkinRepository) GetByUserAndDate(ctx context.Context, userID int64, date string) (*service.CheckinRecord, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, checkin_date, reward_amount, base_reward_amount, bonus_status, bonus_delta_amount, user_timezone, created_at, bonus_played_at
		 FROM checkin_records
		 WHERE user_id = $1 AND checkin_date = $2
		 LIMIT 1`,
		userID,
		date,
	)

	record, err := scanCheckinRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *checkinRepository) ApplyBonusOutcome(ctx context.Context, userID int64, date, outcome string, delta float64) (*service.CheckinRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(
		ctx,
		`UPDATE checkin_records
		 SET reward_amount = reward_amount + $1,
		     bonus_status = $2,
		     bonus_delta_amount = $1,
		     bonus_played_at = NOW()
		 WHERE user_id = $3
		   AND checkin_date = $4
		   AND bonus_status = $5
		 RETURNING id, user_id, checkin_date, reward_amount, base_reward_amount, bonus_status, bonus_delta_amount, user_timezone, created_at, bonus_played_at`,
		delta,
		outcome,
		userID,
		date,
		service.CheckinBonusStatusNone,
	)

	record, err := scanCheckinRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCheckinLuckyBonusAlreadyPlayed
		}
		return nil, err
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE users
		 SET balance = balance + $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL`,
		delta,
		userID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrUserNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &record, nil
}

func (r *checkinRepository) GetUserTotals(ctx context.Context, userID int64) (int64, float64, error) {
	var totalCount int64
	var totalReward string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), COALESCE(SUM(reward_amount), 0)
		 FROM checkin_records
		 WHERE user_id = $1`,
		userID,
	).Scan(&totalCount, &totalReward)
	if err != nil {
		return 0, 0, err
	}
	parsedReward, err := strconv.ParseFloat(totalReward, 64)
	if err != nil {
		return 0, 0, err
	}
	return totalCount, parsedReward, nil
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
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date = $%d", argIndex))
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
		SELECT c.id, c.user_id, u.email, u.username, c.checkin_date, c.reward_amount, c.user_timezone, c.created_at
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
			COALESCE(SUM(c.reward_amount), 0),
			COUNT(*) FILTER (WHERE c.checkin_date = CURRENT_DATE::text),
			COALESCE(AVG(c.reward_amount), 0)
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
	`, whereClause)

	var overview service.AdminCheckinOverview
	var totalReward string
	var avgReward string
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&overview.TotalCheckins,
		&totalReward,
		&overview.TodayCheckins,
		&avgReward,
	)
	if err != nil {
		return service.AdminCheckinOverview{}, err
	}

	overview.TotalRewardAmount, err = strconv.ParseFloat(totalReward, 64)
	if err != nil {
		return service.AdminCheckinOverview{}, err
	}
	overview.AvgRewardAmount, err = strconv.ParseFloat(avgReward, 64)
	if err != nil {
		return service.AdminCheckinOverview{}, err
	}
	return overview, nil
}

func (r *checkinRepository) GetAdminTrend(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTrendPoint, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.checkin_date, COUNT(*), COALESCE(SUM(c.reward_amount), 0)
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
		var reward string
		if err := rows.Scan(&point.Date, &point.CheckinCount, &reward); err != nil {
			return nil, err
		}
		point.RewardAmount, err = strconv.ParseFloat(reward, 64)
		if err != nil {
			return nil, err
		}
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
		SELECT c.reward_amount
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		ORDER BY c.reward_amount ASC
	`, whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rewards := make([]float64, 0)
	for rows.Next() {
		var reward string
		if err := rows.Scan(&reward); err != nil {
			return nil, err
		}
		value, err := strconv.ParseFloat(reward, 64)
		if err != nil {
			return nil, err
		}
		rewards = append(rewards, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(rewards) == 0 {
		return []service.AdminCheckinRewardBucket{}, nil
	}

	minReward := rewards[0]
	maxReward := rewards[len(rewards)-1]
	if len(rewards) == 1 || maxReward <= minReward {
		return []service.AdminCheckinRewardBucket{
			{
				Label:        formatRewardBucketLabel(minReward, maxReward),
				Count:        int64(len(rewards)),
				RewardAmount: sumFloat64s(rewards),
			},
		}, nil
	}

	const bucketCount = 5
	width := (maxReward - minReward) / bucketCount
	buckets := make([]service.AdminCheckinRewardBucket, bucketCount)
	for index := 0; index < bucketCount; index++ {
		start := minReward + float64(index)*width
		end := start + width
		if index == bucketCount-1 {
			end = maxReward
		}
		buckets[index].Label = formatRewardBucketLabel(start, end)
	}

	for _, reward := range rewards {
		bucketIndex := int(math.Floor((reward - minReward) / width))
		if bucketIndex < 0 {
			bucketIndex = 0
		}
		if bucketIndex >= bucketCount {
			bucketIndex = bucketCount - 1
		}
		buckets[bucketIndex].Count++
		buckets[bucketIndex].RewardAmount += reward
	}

	result := make([]service.AdminCheckinRewardBucket, 0, bucketCount)
	for _, bucket := range buckets {
		if bucket.Count == 0 {
			continue
		}
		result = append(result, bucket)
	}
	return result, nil
}

func (r *checkinRepository) GetAdminTopUsers(ctx context.Context, filter service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTopUser, error) {
	whereClause, args := buildAdminCheckinAnalyticsWhere(filter)
	limit := filter.TopLimit
	if limit <= 0 {
		limit = 10
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			u.username,
			COUNT(*),
			COALESCE(SUM(c.reward_amount), 0)
		FROM checkin_records c
		JOIN users u ON u.id = c.user_id
		WHERE %s
		GROUP BY u.id, u.email, u.username
		ORDER BY COUNT(*) DESC, COALESCE(SUM(c.reward_amount), 0) DESC, u.id ASC
		LIMIT $%d
	`, whereClause, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]service.AdminCheckinTopUser, 0, limit)
	for rows.Next() {
		var item service.AdminCheckinTopUser
		var reward string
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.UserName, &item.CheckinCount, &reward); err != nil {
			return nil, err
		}
		item.RewardAmount, err = strconv.ParseFloat(reward, 64)
		if err != nil {
			return nil, err
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

type checkinScanner interface {
	Scan(dest ...any) error
}

func scanCheckinRecord(scanner checkinScanner) (service.CheckinRecord, error) {
	var record service.CheckinRecord
	var reward string
	var baseReward string
	var bonusDelta string
	var bonusPlayedAt sql.NullTime
	err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&record.CheckinDate,
		&reward,
		&baseReward,
		&record.BonusStatus,
		&bonusDelta,
		&record.UserTimezone,
		&record.CreatedAt,
		&bonusPlayedAt,
	)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	record.RewardAmount, err = strconv.ParseFloat(reward, 64)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	record.BaseRewardAmount, err = strconv.ParseFloat(baseReward, 64)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	record.BonusDeltaAmount, err = strconv.ParseFloat(bonusDelta, 64)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	if bonusPlayedAt.Valid {
		record.BonusPlayedAt = &bonusPlayedAt.Time
	}
	return record, nil
}

func scanAdminCheckinRecord(scanner checkinScanner) (service.AdminCheckinRecord, error) {
	var record service.AdminCheckinRecord
	var reward string
	err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&record.UserEmail,
		&record.UserName,
		&record.CheckinDate,
		&reward,
		&record.UserTimezone,
		&record.CreatedAt,
	)
	if err != nil {
		return service.AdminCheckinRecord{}, err
	}
	record.RewardAmount, err = strconv.ParseFloat(reward, 64)
	if err != nil {
		return service.AdminCheckinRecord{}, err
	}
	return record, nil
}

func isCheckinUniqueViolation(err error) bool {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func adminCheckinOrderBy(sortBy, sortOrder string) string {
	field := "c.created_at"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "reward_amount":
		field = "c.reward_amount"
	case "checkin_date":
		field = "c.checkin_date"
	case "user_email":
		field = "u.email"
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

	if filter.StartDate != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date >= $%d", argIndex))
		args = append(args, filter.StartDate)
		argIndex++
	}
	if filter.EndDate != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.checkin_date <= $%d", argIndex))
		args = append(args, filter.EndDate)
		argIndex++
	}

	search := strings.TrimSpace(filter.Search)
	if search != "" {
		whereParts = append(whereParts, fmt.Sprintf("(u.email ILIKE $%d OR u.username ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}

	timezone := strings.TrimSpace(filter.Timezone)
	if timezone != "" {
		whereParts = append(whereParts, fmt.Sprintf("c.user_timezone = $%d", argIndex))
		args = append(args, timezone)
	}

	return strings.Join(whereParts, " AND "), args
}

func formatRewardBucketLabel(start, end float64) string {
	return fmt.Sprintf("%.6f - %.6f", start, end)
}

func sumFloat64s(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}
