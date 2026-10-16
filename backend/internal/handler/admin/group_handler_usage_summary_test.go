package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupUsageSummaryRepoStub struct {
	service.UsageLogRepository
	calls int32
	fail  bool
}

func (r *groupUsageSummaryRepoStub) GetAllGroupUsageSummary(_ context.Context, _ time.Time) ([]usagestats.GroupUsageSummary, error) {
	call := atomic.AddInt32(&r.calls, 1)
	if r.fail {
		return nil, errors.New("database unavailable")
	}
	return []usagestats.GroupUsageSummary{{GroupID: 7, TodayCost: float64(call), YesterdayCost: 2, TotalCost: 9}}, nil
}

func TestGroupHandlerUsageSummaryCompatibilityAndManualRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &groupUsageSummaryRepoStub{}
	h := NewGroupHandler(nil, service.NewDashboardService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/groups/usage-summary", h.GetUsageSummary)
	request := func(query string) *httptest.ResponseRecorder {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/groups/usage-summary"+query, nil))
		return res
	}
	legacy := request("")
	require.Equal(t, http.StatusOK, legacy.Code)
	require.Equal(t, "no-store", legacy.Header().Get("Cache-Control"))
	var legacyBody struct {
		Data []usagestats.GroupUsageSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(legacy.Body.Bytes(), &legacyBody))
	require.Equal(t, float64(1), legacyBody.Data[0].TodayCost)
	var body struct {
		Data service.GroupUsageSummarySnapshot `json:"data"`
	}
	res := request("?include_metadata=true")
	require.Equal(t, http.StatusOK, res.Code)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	firstTimestamp := body.Data.UpdatedAt
	require.False(t, firstTimestamp.IsZero())
	require.False(t, body.Data.Stale)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
	res = request("?include_metadata=true&refresh=true")
	require.Equal(t, http.StatusOK, res.Code)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, float64(2), body.Data.Items[0].TodayCost)
	require.Equal(t, int32(2), atomic.LoadInt32(&repo.calls))
	lastTimestamp := body.Data.UpdatedAt
	repo.fail = true
	res = request("?include_metadata=true&refresh=true")
	require.Equal(t, http.StatusOK, res.Code)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, lastTimestamp, body.Data.UpdatedAt)
	require.Equal(t, float64(2), body.Data.Items[0].TodayCost)
	require.True(t, body.Data.Stale)
	require.True(t, body.Data.RefreshFailed)
	for _, query := range []string{"?refresh=invalid", "?include_metadata=invalid"} {
		require.Equal(t, http.StatusBadRequest, request(query).Code)
	}
}
