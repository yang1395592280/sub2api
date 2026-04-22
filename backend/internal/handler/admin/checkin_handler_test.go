package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminCheckinRepoStub struct {
	items     []service.AdminCheckinRecord
	total     int64
	analytics *service.AdminCheckinAnalytics
	lastQuery struct {
		page      int
		pageSize  int
		search    string
		date      string
		timezone  string
		sortBy    string
		sortOrder string
	}
	lastAnalyticsFilter service.AdminCheckinAnalyticsFilter
}

func (s *adminCheckinRepoStub) HasCheckedInOnDate(_ context.Context, _ int64, _ string) (bool, error) {
	panic("unexpected HasCheckedInOnDate call")
}

func (s *adminCheckinRepoStub) CreateAndCredit(_ context.Context, _ *service.CheckinRecord) (*service.CheckinRecord, error) {
	panic("unexpected CreateAndCredit call")
}

func (s *adminCheckinRepoStub) ListByUserAndDateRange(_ context.Context, _ int64, _, _ string) ([]service.CheckinRecord, error) {
	panic("unexpected ListByUserAndDateRange call")
}

func (s *adminCheckinRepoStub) GetByUserAndDate(_ context.Context, _ int64, _ string) (*service.CheckinRecord, error) {
	panic("unexpected GetByUserAndDate call")
}

func (s *adminCheckinRepoStub) ApplyBonusOutcome(_ context.Context, _ int64, _, _ string, _ float64) (*service.CheckinRecord, error) {
	panic("unexpected ApplyBonusOutcome call")
}

func (s *adminCheckinRepoStub) GetUserTotals(_ context.Context, _ int64) (int64, float64, error) {
	panic("unexpected GetUserTotals call")
}

func (s *adminCheckinRepoStub) ListAdminRecords(_ context.Context, page, pageSize int, search, date, timezone, sortBy, sortOrder string) ([]service.AdminCheckinRecord, int64, error) {
	s.lastQuery.page = page
	s.lastQuery.pageSize = pageSize
	s.lastQuery.search = search
	s.lastQuery.date = date
	s.lastQuery.timezone = timezone
	s.lastQuery.sortBy = sortBy
	s.lastQuery.sortOrder = sortOrder
	return s.items, s.total, nil
}

func (s *adminCheckinRepoStub) GetAdminOverview(_ context.Context, filter service.AdminCheckinAnalyticsFilter) (service.AdminCheckinOverview, error) {
	s.lastAnalyticsFilter = filter
	if s.analytics == nil {
		return service.AdminCheckinOverview{}, nil
	}
	return s.analytics.Overview, nil
}

func (s *adminCheckinRepoStub) GetAdminTrend(_ context.Context, _ service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTrendPoint, error) {
	if s.analytics == nil {
		return nil, nil
	}
	return s.analytics.Trend, nil
}

func (s *adminCheckinRepoStub) GetAdminRewardDistribution(_ context.Context, _ service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinRewardBucket, error) {
	if s.analytics == nil {
		return nil, nil
	}
	return s.analytics.RewardDistribution, nil
}

func (s *adminCheckinRepoStub) GetAdminTopUsers(_ context.Context, _ service.AdminCheckinAnalyticsFilter) ([]service.AdminCheckinTopUser, error) {
	if s.analytics == nil {
		return nil, nil
	}
	return s.analytics.TopUsers, nil
}

func TestAdminCheckinHandlerListPassesFiltersAndReturnsData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminCheckinRepoStub{
		items: []service.AdminCheckinRecord{
			{UserID: 1, UserEmail: "alice@example.com", UserName: "alice", CheckinDate: "2026-04-21", RewardAmount: 18.5},
		},
		total: 1,
	}
	checkinService := service.NewCheckinService(repo, nil, nil, nil)
	handler := NewCheckinHandler(checkinService)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/checkins?page=1&page_size=20&search=alice&date=2026-04-21&timezone=Asia%2FShanghai&sort_by=reward_amount&sort_order=asc", nil)

	handler.List(c)

	require.Equal(t, 1, repo.lastQuery.page)
	require.Equal(t, 20, repo.lastQuery.pageSize)
	require.Equal(t, "alice", repo.lastQuery.search)
	require.Equal(t, "2026-04-21", repo.lastQuery.date)
	require.Equal(t, "Asia/Shanghai", repo.lastQuery.timezone)
	require.Equal(t, "reward_amount", repo.lastQuery.sortBy)
	require.Equal(t, "asc", repo.lastQuery.sortOrder)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []service.AdminCheckinRecord `json:"items"`
			Total int64                        `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, int64(1), resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, "alice@example.com", resp.Data.Items[0].UserEmail)
}

func TestAdminCheckinHandlerAnalyticsPassesFiltersAndReturnsData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminCheckinRepoStub{
		analytics: &service.AdminCheckinAnalytics{
			Overview: service.AdminCheckinOverview{TotalCheckins: 9, TodayCheckins: 2},
			Trend: []service.AdminCheckinTrendPoint{
				{Date: "2026-04-20", CheckinCount: 4, RewardAmount: 0.08},
			},
		},
	}
	checkinService := service.NewCheckinService(repo, nil, nil, nil)
	handler := NewCheckinHandler(checkinService)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/checkins/analytics?start_date=2026-04-01&end_date=2026-04-21&search=alice&timezone=Asia%2FShanghai&top_limit=5", nil)

	handler.Analytics(c)

	require.Equal(t, "2026-04-01", repo.lastAnalyticsFilter.StartDate)
	require.Equal(t, "2026-04-21", repo.lastAnalyticsFilter.EndDate)
	require.Equal(t, "alice", repo.lastAnalyticsFilter.Search)
	require.Equal(t, "Asia/Shanghai", repo.lastAnalyticsFilter.Timezone)
	require.Equal(t, 5, repo.lastAnalyticsFilter.TopLimit)
	require.Equal(t, http.StatusOK, rec.Code)
}
