//go:build unit

package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dailyStatsRepoStub struct {
	query  service.UserDailyStatsQuery
	userID int64
	err    error
}

func (r *dailyStatsRepoStub) GetDailyStats(_ context.Context, q service.UserDailyStatsQuery) (*service.UserDailyStatsResponse, error) {
	r.query = q
	return &service.UserDailyStatsResponse{Items: []service.UserDailyStatsRow{}, Page: q.Page, PageSize: q.PageSize}, r.err
}
func (r *dailyStatsRepoStub) GetDailyActivities(_ context.Context, id int64, q service.UserDailyStatsQuery) (*service.UserDailyActivitiesResponse, error) {
	r.query = q
	r.userID = id
	return &service.UserDailyActivitiesResponse{Items: []service.UserDailyActivity{}, Page: q.Page, PageSize: q.PageSize}, r.err
}

func TestUserDailyStatsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &dailyStatsRepoStub{}
	h := NewUserDailyStatsHandler(service.NewUserDailyStatsService(repo))
	router := gin.New()
	router.GET("/stats", h.List)
	router.GET("/stats/:id/activities", h.Activities)
	request := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	rec := request("/stats?date=2026-11-13&search=alice&activity=liyu&sort_by=liyu_net&sort_order=asc&page=2&page_size=500")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"previous_date":"2026-11-12"`)
	require.Equal(t, "alice", repo.query.Search)
	require.Equal(t, "liyu", repo.query.Activity)
	require.Equal(t, "liyu_net", repo.query.SortBy)
	require.Equal(t, "asc", repo.query.SortOrder)
	require.Equal(t, 2, repo.query.Page)
	require.Equal(t, 100, repo.query.PageSize)
	for _, path := range []string{"/stats?date=oops", "/stats?activity=invalid", "/stats?sort_by=unknown", "/stats/0/activities", "/stats/nope/activities", "/stats/-1/activities"} {
		require.Equal(t, http.StatusBadRequest, request(path).Code, path)
	}
	rec = request("/stats/123/activities?date=2026-11-12&page=3")
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 123, repo.userID)
	require.Equal(t, "2026-11-12", repo.query.Date)
	require.Equal(t, 3, repo.query.Page)
	repo.err = errors.New("query failed")
	require.Equal(t, http.StatusInternalServerError, request("/stats").Code)
	require.Equal(t, http.StatusInternalServerError, request("/stats/123/activities").Code)
}
