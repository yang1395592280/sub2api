package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var createdAt time.Time
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO checkin_records (user_id, checkin_date, reward_amount, user_timezone)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		record.UserID,
		record.CheckinDate,
		record.RewardAmount,
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
		`SELECT id, user_id, checkin_date, reward_amount, user_timezone, created_at
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

func (r *checkinRepository) ListAdminRecords(ctx context.Context, page, pageSize int, search, date, sortBy, sortOrder string) ([]service.AdminCheckinRecord, int64, error) {
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

type checkinScanner interface {
	Scan(dest ...any) error
}

func scanCheckinRecord(scanner checkinScanner) (service.CheckinRecord, error) {
	var record service.CheckinRecord
	var reward string
	err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&record.CheckinDate,
		&reward,
		&record.UserTimezone,
		&record.CreatedAt,
	)
	if err != nil {
		return service.CheckinRecord{}, err
	}
	record.RewardAmount, err = strconv.ParseFloat(reward, 64)
	if err != nil {
		return service.CheckinRecord{}, err
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
