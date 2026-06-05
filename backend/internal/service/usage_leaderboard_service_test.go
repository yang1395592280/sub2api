package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type usageLeaderboardRepoStub struct {
	listItems         []UsageLeaderboardRawItem
	listResult        *pagination.PaginationResult
	listErr           error
	currentUserItem   *UsageLeaderboardRawItem
	currentUserErr    error
	participantCount  int64
	participantCountErr error

	lastListDate       time.Time
	lastListMetric     UsageLeaderboardMetric
	lastListParams     pagination.PaginationParams
	lastCurrentUserID  int64
	lastCurrentDate    time.Time
	lastCurrentMetric  UsageLeaderboardMetric
	lastParticipantDate   time.Time
	lastParticipantMetric UsageLeaderboardMetric
}

func (s *usageLeaderboardRepoStub) ListUsageLeaderboard(_ context.Context, date time.Time, metric UsageLeaderboardMetric, params pagination.PaginationParams) ([]UsageLeaderboardRawItem, *pagination.PaginationResult, error) {
	s.lastListDate = date
	s.lastListMetric = metric
	s.lastListParams = params
	if s.listErr != nil {
		return nil, nil, s.listErr
	}
	if s.listResult == nil {
		s.listResult = &pagination.PaginationResult{
			Total:    int64(len(s.listItems)),
			Page:     params.Page,
			PageSize: params.PageSize,
			Pages:    1,
		}
	}
	return s.listItems, s.listResult, nil
}

func (s *usageLeaderboardRepoStub) GetUsageLeaderboardCurrentUserEntry(_ context.Context, date time.Time, metric UsageLeaderboardMetric, userID int64) (*UsageLeaderboardRawItem, error) {
	s.lastCurrentDate = date
	s.lastCurrentMetric = metric
	s.lastCurrentUserID = userID
	if s.currentUserErr != nil {
		return nil, s.currentUserErr
	}
	return s.currentUserItem, nil
}

func (s *usageLeaderboardRepoStub) CountUsageLeaderboardParticipants(_ context.Context, date time.Time, metric UsageLeaderboardMetric) (int64, error) {
	s.lastParticipantDate = date
	s.lastParticipantMetric = metric
	if s.participantCountErr != nil {
		return 0, s.participantCountErr
	}
	return s.participantCount, nil
}

func TestUsageLeaderboardServiceGetOverviewMasksEntriesAndBuildsSummary(t *testing.T) {
	t.Parallel()

	repo := &usageLeaderboardRepoStub{
		listItems: []UsageLeaderboardRawItem{
			{Rank: 1, UserID: 1, Username: "alice", Email: "alice@example.com", Requests: 12, Tokens: 120},
			{Rank: 2, UserID: 7, Username: "bo", Email: "bo@example.com", Requests: 10, Tokens: 100},
			{Rank: 3, UserID: 9, Username: "charlie", Email: "charlie@example.com", Requests: 8, Tokens: 80},
		},
		currentUserItem:  &UsageLeaderboardRawItem{Rank: 2, UserID: 7, Username: "bo", Email: "bo@example.com", Requests: 10, Tokens: 100},
		participantCount: 25,
	}
	svc := NewUsageLeaderboardService(repo)

	result, err := svc.GetOverview(context.Background(), 7, UsageLeaderboardQuery{
		Date:   "2026-06-01",
		Metric: "requests",
	})
	require.NoError(t, err)
	require.Equal(t, "2026-06-01", result.Date)
	require.Equal(t, string(UsageLeaderboardMetricRequests), result.Metric)
	require.Equal(t, int64(25), result.ParticipantCount)
	require.Len(t, result.TopItems, 3)
	require.Equal(t, int64(12), result.TopItems[0].Value)
	require.Equal(t, "a***e", result.TopItems[0].Username)
	require.Equal(t, "a***@e***.com", result.TopItems[0].Email)
	require.NotNil(t, result.CurrentUser)
	require.Equal(t, int64(2), result.CurrentUser.Rank)
	require.True(t, result.CurrentUser.IsCurrentUser)
	require.Equal(t, "b*", result.CurrentUser.Username)
	require.Equal(t, "b***@e***.com", result.CurrentUser.Email)
	require.Equal(t, int64(7), repo.lastCurrentUserID)
	require.Equal(t, UsageLeaderboardMetricRequests, repo.lastListMetric)
	require.Equal(t, 3, repo.lastListParams.PageSize)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), repo.lastListDate)
}

func TestUsageLeaderboardServiceGetItemsUsesTokensMetricValue(t *testing.T) {
	t.Parallel()

	repo := &usageLeaderboardRepoStub{
		listItems: []UsageLeaderboardRawItem{
			{Rank: 1, UserID: 10, Username: "david", Email: "david@example.com", Requests: 3, Tokens: 300},
			{Rank: 2, UserID: 11, Username: "eve", Email: "eve@example.com", Requests: 2, Tokens: 200},
		},
		listResult: &pagination.PaginationResult{Total: 2, Page: 2, PageSize: 1, Pages: 2},
	}
	svc := NewUsageLeaderboardService(repo)

	items, result, err := svc.GetItems(context.Background(), 99, UsageLeaderboardQuery{
		Date:   "2026-06-02",
		Metric: "tokens",
	}, pagination.PaginationParams{Page: 2, PageSize: 1})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.NotNil(t, result)
	require.Equal(t, int64(300), items[0].Value)
	require.Equal(t, string(UsageLeaderboardMetricTokens), items[0].Metric)
	require.Equal(t, UsageLeaderboardMetricTokens, repo.lastListMetric)
	require.Equal(t, 2, repo.lastListParams.Page)
	require.Equal(t, 1, repo.lastListParams.PageSize)
}

func TestUsageLeaderboardServiceRejectsInvalidMetric(t *testing.T) {
	t.Parallel()

	svc := NewUsageLeaderboardService(&usageLeaderboardRepoStub{})

	_, err := svc.GetOverview(context.Background(), 7, UsageLeaderboardQuery{
		Date:   "2026-06-01",
		Metric: "cost",
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestUsageLeaderboardServiceRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	svc := NewUsageLeaderboardService(&usageLeaderboardRepoStub{})

	_, _, err := svc.GetItems(context.Background(), 7, UsageLeaderboardQuery{
		Date:   "2026/06/01",
		Metric: "requests",
	}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestUsageLeaderboardMaskHelpers(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", maskUsername(""))
	require.Equal(t, "*", maskUsername("a"))
	require.Equal(t, "b*", maskUsername("bo"))
	require.Equal(t, "c*r", maskUsername("car"))
	require.Equal(t, "a***e", maskUsername("alice"))
	require.Equal(t, "张*丰", maskUsername("张三丰"))
	require.Equal(t, "", maskEmail(""))
	require.Equal(t, "a***@e***.com", maskEmail("alice@example.com"))
	require.Equal(t, "b***@e***.com", maskEmail("bo@example.com"))
}
