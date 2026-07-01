package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubBusinessAnalyticsService struct {
	overview        *service.BusinessOverviewResponse
	impact          *service.PriceChangeImpactResponse
	overviewFilters []service.BusinessAnalyticsFilter
	groupFilters    []service.BusinessAnalyticsFilter
}

func (s *stubBusinessAnalyticsService) GetOverview(_ context.Context, filter service.BusinessAnalyticsFilter) (*service.BusinessOverviewResponse, error) {
	s.overviewFilters = append(s.overviewFilters, filter)
	if s.overview != nil {
		return s.overview, nil
	}
	return &service.BusinessOverviewResponse{}, nil
}

func (s *stubBusinessAnalyticsService) GetGroups(_ context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessGroupRow, error) {
	s.groupFilters = append(s.groupFilters, filter)
	return nil, nil
}

func (s *stubBusinessAnalyticsService) GetChannels(context.Context, service.BusinessAnalyticsFilter) ([]service.BusinessChannelRow, error) {
	return nil, nil
}

func (s *stubBusinessAnalyticsService) GetPriceChangeImpact(context.Context, service.PriceChangeImpactInput) (*service.PriceChangeImpactResponse, error) {
	if s.impact != nil {
		return s.impact, nil
	}
	return &service.PriceChangeImpactResponse{}, nil
}

func (s *stubBusinessAnalyticsService) GetRecords(context.Context, service.BusinessRecordsFilter) (*service.BusinessRecordsResponse, error) {
	return &service.BusinessRecordsResponse{}, nil
}

func TestBusinessAnalyticsHandler_OverviewRequiresValidDateRange(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{})

	for _, target := range []string{
		"/overview",
		"/overview?start_date=bad&end_date=2026-06-02",
		"/overview?start_date=2026-06-03&end_date=2026-06-02",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, target)
	}
}

func TestBusinessAnalyticsHandler_OverviewReturnsProfitMetrics(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{
		overview: &service.BusinessOverviewResponse{
			StartDate:    "2026-06-01",
			EndDate:      "2026-06-02",
			Revenue:      10,
			ChannelCost:  6,
			GrossProfit:  4,
			ProfitMargin: float64Ptr(0.4),
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/overview?start_date=2026-06-01&end_date=2026-06-02", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, float64(4), envelope.Data["gross_profit"])
	require.Equal(t, float64(0.4), envelope.Data["profit_margin"])
}

func TestBusinessAnalyticsHandler_OverviewPassesEndDateAsExclusiveNextDay(t *testing.T) {
	svc := &stubBusinessAnalyticsService{}
	router := businessAnalyticsTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/overview?start_date=2026-06-01&end_date=2026-06-02", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, svc.overviewFilters, 1)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), svc.overviewFilters[0].StartDate)
	require.Equal(t, time.Date(2026, 6, 3, 0, 0, 0, 0, time.Local), svc.overviewFilters[0].EndDate)
}

func TestBusinessAnalyticsHandler_ChannelGroupsPathIDIsAccountID(t *testing.T) {
	svc := &stubBusinessAnalyticsService{}
	router := businessAnalyticsTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/channels/123/groups?start_date=2026-06-01&end_date=2026-06-02", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, svc.groupFilters, 1)
	require.Equal(t, int64(123), svc.groupFilters[0].AccountID)
	require.Zero(t, svc.groupFilters[0].GroupID)
}

func TestBusinessAnalyticsHandler_PriceChangeImpactRequiresGroupAndChangeDate(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{})

	for _, target := range []string{
		"/price-change-impact?change_date=2026-06-01",
		"/price-change-impact?group_id=1",
		"/price-change-impact?group_id=1&change_date=bad",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, target)
	}
}

func TestBusinessAnalyticsHandler_PriceChangeImpactReturnsImpact(t *testing.T) {
	changeDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{
		impact: &service.PriceChangeImpactResponse{
			GroupID:       1,
			ChangeDate:    "2026-06-01",
			BeforeRevenue: 8,
			AfterRevenue:  12,
			RevenueDelta:  4,
			ChangeAt:      changeDate,
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/price-change-impact?group_id=1&change_date=2026-06-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, float64(1), envelope.Data["group_id"])
	require.Equal(t, "2026-06-01", envelope.Data["change_date"])
	require.Equal(t, float64(4), envelope.Data["revenue_delta"])
}

func businessAnalyticsTestRouter(svc businessAnalyticsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewBusinessAnalyticsHandler(svc)
	router := gin.New()
	router.GET("/overview", h.GetOverview)
	router.GET("/channels/:id/groups", h.GetChannelGroups)
	router.GET("/price-change-impact", h.GetPriceChangeImpact)
	return router
}

func float64Ptr(v float64) *float64 {
	return &v
}
