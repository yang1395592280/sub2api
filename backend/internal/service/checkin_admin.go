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

type AdminCheckinAnalyticsFilter struct {
	StartDate string
	EndDate   string
	Search    string
	Timezone  string
	TopLimit  int
}

type AdminCheckinOverview struct {
	TotalCheckins     int64   `json:"total_checkins"`
	TotalRewardAmount float64 `json:"total_reward_amount"`
	TodayCheckins     int64   `json:"today_checkins"`
	AvgRewardAmount   float64 `json:"avg_reward_amount"`
}

type AdminCheckinTrendPoint struct {
	Date         string  `json:"date"`
	CheckinCount int64   `json:"checkin_count"`
	RewardAmount float64 `json:"reward_amount"`
}

type AdminCheckinRewardBucket struct {
	Label        string  `json:"label"`
	Count        int64   `json:"count"`
	RewardAmount float64 `json:"reward_amount"`
}

type AdminCheckinTopUser struct {
	UserID       int64   `json:"user_id"`
	UserEmail    string  `json:"user_email"`
	UserName     string  `json:"user_name"`
	CheckinCount int64   `json:"checkin_count"`
	RewardAmount float64 `json:"reward_amount"`
}

type AdminCheckinAnalytics struct {
	Overview           AdminCheckinOverview       `json:"overview"`
	Trend              []AdminCheckinTrendPoint   `json:"trend"`
	RewardDistribution []AdminCheckinRewardBucket `json:"reward_distribution"`
	TopUsers           []AdminCheckinTopUser      `json:"top_users"`
}

func (s *CheckinService) ListAdminRecords(ctx context.Context, page, pageSize int, search, date, timezone, sortBy, sortOrder string) ([]AdminCheckinRecord, int64, error) {
	return s.repo.ListAdminRecords(ctx, page, pageSize, search, date, timezone, sortBy, sortOrder)
}

func (s *CheckinService) GetAdminAnalytics(ctx context.Context, filter AdminCheckinAnalyticsFilter) (*AdminCheckinAnalytics, error) {
	if filter.TopLimit <= 0 {
		filter.TopLimit = 10
	}

	overview, err := s.repo.GetAdminOverview(ctx, filter)
	if err != nil {
		return nil, err
	}
	trend, err := s.repo.GetAdminTrend(ctx, filter)
	if err != nil {
		return nil, err
	}
	distribution, err := s.repo.GetAdminRewardDistribution(ctx, filter)
	if err != nil {
		return nil, err
	}
	topUsers, err := s.repo.GetAdminTopUsers(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &AdminCheckinAnalytics{
		Overview:           overview,
		Trend:              trend,
		RewardDistribution: distribution,
		TopUsers:           topUsers,
	}, nil
}
