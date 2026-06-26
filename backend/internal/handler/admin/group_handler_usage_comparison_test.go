package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupMemberUsageComparisonRepoStub struct {
	service.UsageLogRepository

	todayStart time.Time
}

func (s *groupMemberUsageComparisonRepoStub) GetGroupUserDailyStatsBatch(_ context.Context, _ int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	stats := make(map[int64]*usagestats.AccountStats, len(userIDs))
	for _, userID := range userIDs {
		stats[userID] = &usagestats.AccountStats{}
	}

	switch {
	case startTime.Equal(s.todayStart) && endTime.Equal(s.todayStart.AddDate(0, 0, 1)):
		stats[1] = &usagestats.AccountStats{
			Requests:     2,
			Tokens:       100,
			Cost:         3.75,
			StandardCost: 3.75,
			UserCost:     2.25,
		}
	case startTime.Equal(s.todayStart.AddDate(0, 0, -1)) && endTime.Equal(s.todayStart):
		stats[1] = &usagestats.AccountStats{
			Requests:     1,
			Tokens:       40,
			Cost:         1.25,
			StandardCost: 1.25,
			UserCost:     0.75,
		}
	}

	return stats, nil
}

func newGroupMemberUsageComparisonRouter(adminSvc service.AdminService, repo service.UsageLogRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewGroupHandler(adminSvc, service.NewDashboardService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/api/v1/admin/groups/:id/members/usage-comparison", handler.GetGroupMemberUsageComparison)
	return router
}

func TestGroupHandler_GetGroupMemberUsageComparison_Success(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.groups = []service.Group{
		{ID: 10, Name: "exclusive", IsExclusive: true, Status: service.StatusActive},
	}
	todayStart := timezone.StartOfDayInUserLocation(timezone.NowInUserLocation("UTC"), "UTC")
	router := newGroupMemberUsageComparisonRouter(adminSvc, &groupMemberUsageComparisonRepoStub{todayStart: todayStart})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/10/members/usage-comparison?user_ids=1,2&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			GroupID   int64  `json:"group_id"`
			Today     string `json:"today"`
			Yesterday string `json:"yesterday"`
			Stats     map[string]struct {
				Today struct {
					Requests     int64   `json:"requests"`
					Tokens       int64   `json:"tokens"`
					Cost         float64 `json:"cost"`
					StandardCost float64 `json:"standard_cost"`
					UserCost     float64 `json:"user_cost"`
				} `json:"today"`
				Yesterday struct {
					Requests     int64   `json:"requests"`
					Tokens       int64   `json:"tokens"`
					Cost         float64 `json:"cost"`
					StandardCost float64 `json:"standard_cost"`
					UserCost     float64 `json:"user_cost"`
				} `json:"yesterday"`
			} `json:"stats"`
		} `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 0, resp.Code)
	require.Equal(t, int64(10), resp.Data.GroupID)
	require.NotEmpty(t, resp.Data.Today)
	require.NotEmpty(t, resp.Data.Yesterday)
	require.Len(t, resp.Data.Stats, 2)
	require.Equal(t, int64(2), resp.Data.Stats["1"].Today.Requests)
	require.Equal(t, int64(100), resp.Data.Stats["1"].Today.Tokens)
	require.InDelta(t, 3.75, resp.Data.Stats["1"].Today.Cost, 0.001)
	require.InDelta(t, 3.75, resp.Data.Stats["1"].Today.StandardCost, 0.001)
	require.InDelta(t, 2.25, resp.Data.Stats["1"].Today.UserCost, 0.001)
	require.Equal(t, int64(1), resp.Data.Stats["1"].Yesterday.Requests)
	require.Equal(t, int64(40), resp.Data.Stats["1"].Yesterday.Tokens)
	require.InDelta(t, 1.25, resp.Data.Stats["1"].Yesterday.Cost, 0.001)
	require.InDelta(t, 1.25, resp.Data.Stats["1"].Yesterday.StandardCost, 0.001)
	require.InDelta(t, 0.75, resp.Data.Stats["1"].Yesterday.UserCost, 0.001)
	require.Equal(t, int64(0), resp.Data.Stats["2"].Today.Requests)
	require.Equal(t, int64(0), resp.Data.Stats["2"].Yesterday.Requests)
}

func TestGroupHandler_GetGroupMemberUsageComparison_NonExclusiveForbidden(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.groups = []service.Group{
		{ID: 11, Name: "shared", IsExclusive: false, Status: service.StatusActive},
	}
	todayStart := timezone.StartOfDayInUserLocation(timezone.NowInUserLocation("UTC"), "UTC")
	router := newGroupMemberUsageComparisonRouter(adminSvc, &groupMemberUsageComparisonRepoStub{todayStart: todayStart})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/11/members/usage-comparison?user_ids=1&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.NotEmpty(t, resp.Message)
	require.Equal(t, "GROUP_USAGE_EXCLUSIVE_ONLY", resp.Reason)
}

func TestGroupHandler_GetGroupMemberUsageComparison_InvalidUserIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	router := newGroupMemberUsageComparisonRouter(adminSvc, &groupMemberUsageComparisonRepoStub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/10/members/usage-comparison?user_ids=1,abc&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "Invalid user_ids", resp.Message)
}
