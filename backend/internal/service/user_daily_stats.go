package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const UserDailyStatsTimezone = "Asia/Shanghai"

type UserDailyUsage struct {
	TotalCost        float64 `json:"total_cost"`
	PaidCost         float64 `json:"paid_cost"`
	WelfareCost      float64 `json:"welfare_cost"`
	CheckinCost      float64 `json:"checkin_cost"`
	LiyuCost         float64 `json:"liyu_cost"`
	GiftCost         float64 `json:"gift_cost"`
	SubscriptionCost float64 `json:"subscription_cost"`
	UnclassifiedCost float64 `json:"unclassified_cost"`
	Requests         int64   `json:"requests"`
	Tokens           int64   `json:"tokens"`
}

type UserDailyActivityStats struct {
	EventCount          int64   `json:"event_count"`
	CheckinEvents       int64   `json:"checkin_events"`
	CheckinCount        int64   `json:"checkin_count"`
	CheckinBase         float64 `json:"checkin_base"`
	CheckinBonus        float64 `json:"checkin_bonus"`
	CheckinNet          float64 `json:"checkin_net"`
	LiyuDraws           int64   `json:"liyu_draws"`
	LiyuEvents          int64   `json:"liyu_events"`
	LiyuReward          float64 `json:"liyu_reward"`
	LiyuLucky           float64 `json:"liyu_lucky"`
	LiyuGuess           float64 `json:"liyu_guess"`
	LiyuNet             float64 `json:"liyu_net"`
	LiyuTicketFaceValue float64 `json:"liyu_ticket_face_value"`
	GiftCredit          float64 `json:"gift_credit"`
	WelfareAdjustment   float64 `json:"welfare_adjustment"`
}

type UserDailyStatsRow struct {
	UserID         int64                  `json:"user_id"`
	Email          string                 `json:"email"`
	Username       string                 `json:"username"`
	Status         string                 `json:"status"`
	Deleted        bool                   `json:"deleted"`
	Balance        float64                `json:"balance"`
	WelfareBalance float64                `json:"welfare_balance"`
	CheckinBalance float64                `json:"checkin_balance"`
	LiyuBalance    float64                `json:"liyu_balance"`
	GiftBalance    float64                `json:"gift_balance"`
	FrozenBalance  float64                `json:"frozen_balance"`
	TotalRecharged float64                `json:"total_recharged"`
	SpendingRank   *int64                 `json:"spending_rank"`
	TodayUsage     UserDailyUsage         `json:"today_usage"`
	PreviousUsage  UserDailyUsage         `json:"previous_usage"`
	Activity       UserDailyActivityStats `json:"activity"`
}

type UserDailyStatsSummary struct {
	Users          int64                  `json:"users"`
	ActiveUsers    int64                  `json:"active_users"`
	CheckinUsers   int64                  `json:"checkin_users"`
	LiyuUsers      int64                  `json:"liyu_users"`
	SpendingUsers  int64                  `json:"spending_users"`
	Balance        float64                `json:"balance"`
	WelfareBalance float64                `json:"welfare_balance"`
	TodayUsage     UserDailyUsage         `json:"today_usage"`
	PreviousUsage  UserDailyUsage         `json:"previous_usage"`
	Activity       UserDailyActivityStats `json:"activity"`
}

type UserDailyStatsResponse struct {
	Date         string                `json:"date"`
	PreviousDate string                `json:"previous_date"`
	Timezone     string                `json:"timezone"`
	GeneratedAt  time.Time             `json:"generated_at"`
	Items        []UserDailyStatsRow   `json:"items"`
	Summary      UserDailyStatsSummary `json:"summary"`
	Total        int64                 `json:"total"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"page_size"`
	Pages        int                   `json:"pages"`
}

type UserDailyActivity struct {
	Kind            string    `json:"kind"`
	RecordID        int64     `json:"record_id"`
	Amount          float64   `json:"amount"`
	TicketFaceValue float64   `json:"ticket_face_value"`
	Outcome         string    `json:"outcome"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserDailyActivitiesResponse struct {
	Items    []UserDailyActivity `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Pages    int                 `json:"pages"`
}

type UserDailyStatsFilter struct {
	Date      string
	Search    string
	Activity  string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

type UserDailyStatsQuery struct {
	UserDailyStatsFilter
	Start         time.Time
	End           time.Time
	PreviousStart time.Time
}

type UserDailyStatsRepository interface {
	GetDailyStats(context.Context, UserDailyStatsQuery) (*UserDailyStatsResponse, error)
	GetDailyActivities(context.Context, int64, UserDailyStatsQuery) (*UserDailyActivitiesResponse, error)
}

type UserDailyStatsService struct {
	repo UserDailyStatsRepository
}

func NewUserDailyStatsService(repo UserDailyStatsRepository) *UserDailyStatsService {
	return &UserDailyStatsService{repo: repo}
}

func (s *UserDailyStatsService) Query(ctx context.Context, filter UserDailyStatsFilter) (*UserDailyStatsResponse, error) {
	now := time.Now()
	query, err := normalizeUserDailyStatsQuery(filter, now)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.repo.GetDailyStats(ctx, query)
	if err != nil {
		return nil, err
	}
	result.Date, result.PreviousDate = query.Date, query.PreviousStart.Format(time.DateOnly)
	result.Timezone, result.GeneratedAt = UserDailyStatsTimezone, now
	return result, nil
}

func (s *UserDailyStatsService) Activities(ctx context.Context, userID int64, filter UserDailyStatsFilter) (*UserDailyActivitiesResponse, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_ID", "Invalid user ID")
	}
	query, err := normalizeUserDailyStatsQuery(filter, time.Now())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return s.repo.GetDailyActivities(ctx, userID, query)
}

func normalizeUserDailyStatsQuery(filter UserDailyStatsFilter, now time.Time) (UserDailyStatsQuery, error) {
	location, err := time.LoadLocation(UserDailyStatsTimezone)
	if err != nil {
		return UserDailyStatsQuery{}, err
	}
	filter.Date = strings.TrimSpace(filter.Date)
	if filter.Date == "" {
		filter.Date = now.In(location).Format(time.DateOnly)
	}
	start, err := time.ParseInLocation(time.DateOnly, filter.Date, location)
	if err != nil || start.Format(time.DateOnly) != filter.Date || start.AddDate(0, 0, -1).Year() < 1 {
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_DATE", "Date must use YYYY-MM-DD")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if utf8.RuneCountInString(filter.Search) > 200 {
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_SEARCH", "Search must not exceed 200 characters")
	}
	switch filter.Activity {
	case "", "all":
		filter.Activity = "all"
	case "active", "checkin", "liyu", "usage":
	default:
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_ACTIVITY", "Invalid activity filter")
	}
	switch filter.SortBy {
	case "":
		filter.SortBy = "today_cost"
	case "today_cost", "previous_cost", "checkin_net", "liyu_net", "balance", "welfare_balance", "requests":
	default:
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_SORT", "Invalid sort column")
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}
	if filter.SortOrder != "asc" && filter.SortOrder != "desc" {
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_SORT", "Invalid sort order")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if filter.Page > 1000000 {
		return UserDailyStatsQuery{}, infraerrors.BadRequest("INVALID_STATS_PAGE", "Page is out of range")
	}
	return UserDailyStatsQuery{UserDailyStatsFilter: filter, Start: start, End: start.AddDate(0, 0, 1), PreviousStart: start.AddDate(0, 0, -1)}, nil
}
