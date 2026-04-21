package service

import (
	"context"
	"time"
)

type AdminCheckinRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	UserName     string    `json:"user_name"`
	CheckinDate  string    `json:"checkin_date"`
	RewardAmount float64   `json:"reward_amount"`
	UserTimezone string    `json:"user_timezone"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *CheckinService) ListAdminRecords(ctx context.Context, page, pageSize int, search, date, sortBy, sortOrder string) ([]AdminCheckinRecord, int64, error) {
	return s.repo.ListAdminRecords(ctx, page, pageSize, search, date, sortBy, sortOrder)
}
