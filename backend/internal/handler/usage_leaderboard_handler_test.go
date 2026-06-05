package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageLeaderboardHandlerStub struct {
	overview      *service.UsageLeaderboardOverview
	overviewErr   error
	items         []service.UsageLeaderboardItem
	itemsResult   *pagination.PaginationResult
	itemsErr      error
	lastUserID    int64
	lastQuery     service.UsageLeaderboardQuery
	lastPage      pagination.PaginationParams
}

func (s *usageLeaderboardHandlerStub) GetOverview(_ context.Context, userID int64, query service.UsageLeaderboardQuery) (*service.UsageLeaderboardOverview, error) {
	s.lastUserID = userID
	s.lastQuery = query
	return s.overview, s.overviewErr
}

func (s *usageLeaderboardHandlerStub) GetItems(_ context.Context, userID int64, query service.UsageLeaderboardQuery, params pagination.PaginationParams) ([]service.UsageLeaderboardItem, *pagination.PaginationResult, error) {
	s.lastUserID = userID
	s.lastQuery = query
	s.lastPage = params
	return s.items, s.itemsResult, s.itemsErr
}

func TestUsageLeaderboardHandlerOverviewRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &UsageLeaderboardHandler{service: &usageLeaderboardHandlerStub{}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage-leaderboard/overview", nil)

	h.GetOverview(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUsageLeaderboardHandlerOverviewPassesQueryAndReturnsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &usageLeaderboardHandlerStub{
		overview: &service.UsageLeaderboardOverview{
			Date:   "2026-06-01",
			Metric: "requests",
			TopItems: []service.UsageLeaderboardItem{
				{Rank: 1, UserID: 7, Username: "a***e", Email: "a***@e***.com", Requests: 12, Tokens: 120, Value: 12, Metric: "requests", IsCurrentUser: true},
			},
		},
	}
	h := &UsageLeaderboardHandler{service: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage-leaderboard/overview?metric=requests&date=2026-06-01", nil)

	h.GetOverview(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(7), stub.lastUserID)
	require.Equal(t, "requests", stub.lastQuery.Metric)
	require.Equal(t, "2026-06-01", stub.lastQuery.Date)
	require.Contains(t, recorder.Body.String(), `"metric":"requests"`)
	require.Contains(t, recorder.Body.String(), `"username":"a***e"`)
}

func TestUsageLeaderboardHandlerItemsReturnsPaginatedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &usageLeaderboardHandlerStub{
		items: []service.UsageLeaderboardItem{
			{Rank: 2, UserID: 8, Username: "b*", Email: "b***@e***.com", Requests: 10, Tokens: 100, Value: 100, Metric: "tokens"},
		},
		itemsResult: &pagination.PaginationResult{Total: 12, Page: 2, PageSize: 5, Pages: 3},
	}
	h := &UsageLeaderboardHandler{service: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage-leaderboard/items?metric=tokens&date=2026-06-01&page=2&page_size=5", nil)

	h.GetItems(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "tokens", stub.lastQuery.Metric)
	require.Equal(t, "2026-06-01", stub.lastQuery.Date)
	require.Equal(t, 2, stub.lastPage.Page)
	require.Equal(t, 5, stub.lastPage.PageSize)
	require.Contains(t, recorder.Body.String(), `"page":2`)
	require.Contains(t, recorder.Body.String(), `"page_size":5`)
}

func TestUsageLeaderboardHandlerItemsMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &usageLeaderboardHandlerStub{
		itemsErr: infraerrors.BadRequest("USAGE_LEADERBOARD_INVALID_METRIC", "metric must be requests or tokens"),
	}
	h := &UsageLeaderboardHandler{service: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage-leaderboard/items?metric=bad", nil)

	h.GetItems(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"reason":"USAGE_LEADERBOARD_INVALID_METRIC"`)
}
