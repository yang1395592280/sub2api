package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type UsageLeaderboardMetric string

const (
	UsageLeaderboardMetricRequests UsageLeaderboardMetric = "requests"
	UsageLeaderboardMetricTokens   UsageLeaderboardMetric = "tokens"
)

type UsageLeaderboardQuery struct {
	Date   string `json:"date"`
	Metric string `json:"metric"`
}

type UsageLeaderboardRawItem struct {
	Rank     int64  `json:"rank"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

type UsageLeaderboardItem struct {
	Rank          int64  `json:"rank"`
	UserID        int64  `json:"user_id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	Requests      int64  `json:"requests"`
	Tokens        int64  `json:"tokens"`
	Value         int64  `json:"value"`
	Metric        string `json:"metric"`
	IsCurrentUser bool   `json:"is_current_user"`
}

type UsageLeaderboardOverview struct {
	Date             string                 `json:"date"`
	Metric           string                 `json:"metric"`
	ParticipantCount int64                  `json:"participant_count"`
	CurrentUser      *UsageLeaderboardItem  `json:"current_user,omitempty"`
	TopItems         []UsageLeaderboardItem `json:"top_items"`
}

type UsageLeaderboardRepository interface {
	ListUsageLeaderboard(ctx context.Context, date time.Time, metric UsageLeaderboardMetric, params pagination.PaginationParams) ([]UsageLeaderboardRawItem, *pagination.PaginationResult, error)
	GetUsageLeaderboardCurrentUserEntry(ctx context.Context, date time.Time, metric UsageLeaderboardMetric, userID int64) (*UsageLeaderboardRawItem, error)
	CountUsageLeaderboardParticipants(ctx context.Context, date time.Time, metric UsageLeaderboardMetric) (int64, error)
}

type UsageLeaderboardService struct {
	repo UsageLeaderboardRepository
	now  func() time.Time
}

func NewUsageLeaderboardService(repo UsageLeaderboardRepository) *UsageLeaderboardService {
	return &UsageLeaderboardService{
		repo: repo,
		now:  timezone.Now,
	}
}

func (s *UsageLeaderboardService) GetOverview(ctx context.Context, userID int64, query UsageLeaderboardQuery) (*UsageLeaderboardOverview, error) {
	date, metric, err := s.normalizeQuery(query)
	if err != nil {
		return nil, err
	}

	items, _, err := s.repo.ListUsageLeaderboard(ctx, date, metric, pagination.PaginationParams{
		Page:      1,
		PageSize:  3,
		SortBy:    string(metric),
		SortOrder: pagination.SortOrderDesc,
	})
	if err != nil {
		return nil, err
	}

	participantCount, err := s.repo.CountUsageLeaderboardParticipants(ctx, date, metric)
	if err != nil {
		return nil, err
	}

	currentUser, err := s.repo.GetUsageLeaderboardCurrentUserEntry(ctx, date, metric, userID)
	if err != nil {
		return nil, err
	}

	topItems := make([]UsageLeaderboardItem, 0, len(items))
	for _, item := range items {
		topItems = append(topItems, buildUsageLeaderboardItem(item, metric, userID))
	}

	var current *UsageLeaderboardItem
	if currentUser != nil {
		item := buildUsageLeaderboardItem(*currentUser, metric, userID)
		current = &item
	}

	return &UsageLeaderboardOverview{
		Date:             date.Format("2006-01-02"),
		Metric:           string(metric),
		ParticipantCount: participantCount,
		CurrentUser:      current,
		TopItems:         topItems,
	}, nil
}

func (s *UsageLeaderboardService) GetItems(ctx context.Context, userID int64, query UsageLeaderboardQuery, params pagination.PaginationParams) ([]UsageLeaderboardItem, *pagination.PaginationResult, error) {
	date, metric, err := s.normalizeQuery(query)
	if err != nil {
		return nil, nil, err
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	params.SortBy = string(metric)
	params.SortOrder = params.NormalizedSortOrder(pagination.SortOrderDesc)

	rows, result, err := s.repo.ListUsageLeaderboard(ctx, date, metric, params)
	if err != nil {
		return nil, nil, err
	}

	items := make([]UsageLeaderboardItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildUsageLeaderboardItem(row, metric, userID))
	}
	return items, result, nil
}

func (s *UsageLeaderboardService) normalizeQuery(query UsageLeaderboardQuery) (time.Time, UsageLeaderboardMetric, error) {
	metric, err := parseUsageLeaderboardMetric(query.Metric)
	if err != nil {
		return time.Time{}, "", err
	}

	dateRaw := strings.TrimSpace(query.Date)
	if dateRaw == "" {
		dateRaw = s.now().Format("2006-01-02")
	}
	date, err := timezone.ParseInLocation("2006-01-02", dateRaw)
	if err != nil {
		return time.Time{}, "", infraerrors.BadRequest("USAGE_LEADERBOARD_INVALID_DATE", "date must be in YYYY-MM-DD format")
	}
	return timezone.StartOfDay(date), metric, nil
}

func parseUsageLeaderboardMetric(raw string) (UsageLeaderboardMetric, error) {
	switch UsageLeaderboardMetric(strings.TrimSpace(raw)) {
	case "", UsageLeaderboardMetricRequests:
		return UsageLeaderboardMetricRequests, nil
	case UsageLeaderboardMetricTokens:
		return UsageLeaderboardMetricTokens, nil
	default:
		return "", infraerrors.BadRequest("USAGE_LEADERBOARD_INVALID_METRIC", "metric must be requests or tokens")
	}
}

func buildUsageLeaderboardItem(raw UsageLeaderboardRawItem, metric UsageLeaderboardMetric, currentUserID int64) UsageLeaderboardItem {
	value := raw.Requests
	if metric == UsageLeaderboardMetricTokens {
		value = raw.Tokens
	}
	return UsageLeaderboardItem{
		Rank:          raw.Rank,
		UserID:        raw.UserID,
		Username:      MaskUsernameForLeaderboard(raw.Username, raw.UserID),
		Email:         MaskEmailForLeaderboard(raw.Email),
		Requests:      raw.Requests,
		Tokens:        raw.Tokens,
		Value:         value,
		Metric:        string(metric),
		IsCurrentUser: raw.UserID > 0 && raw.UserID == currentUserID,
	}
}

func maskUsername(username string) string {
	return MaskUsernameForLeaderboard(username, 0)
}

func MaskUsernameForLeaderboard(username string, userID int64) string {
	username = strings.TrimSpace(username)
	if username == "" && userID > 0 {
		username = "user-" + strconv.FormatInt(userID, 10)
	}
	runes := []rune(username)
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(runes[:1]) + "*"
	case 3:
		return string(runes[:1]) + "*" + string(runes[2:])
	default:
		return string(runes[:1]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1:])
	}
}

func MaskEmailForLeaderboard(email string) string {
	return maskEmail(email)
}
