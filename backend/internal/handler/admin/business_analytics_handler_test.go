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
	records         *service.BusinessRecordsResponse
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
	if s.records != nil {
		return s.records, nil
	}
	return &service.BusinessRecordsResponse{}, nil
}

func TestBusinessAnalyticsHandler_OverviewRequiresValidDateRange(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{})

	for _, tt := range []struct {
		target  string
		message string
	}{
		{"/overview", "start_date 和 end_date 为必填项"},
		{"/overview?start_date=bad&end_date=2026-06-02", "start_date 格式无效，请使用 YYYY-MM-DD"},
		{"/overview?start_date=2026-06-01&end_date=bad", "end_date 格式无效，请使用 YYYY-MM-DD"},
		{"/overview?start_date=2026-06-03&end_date=2026-06-02", "end_date 必须大于或等于 start_date"},
		{"/overview?start_date=2026-06-01&end_date=2026-06-02&group_id=abc", "group_id 无效"},
		{"/overview?start_date=2026-06-01&end_date=2026-06-02&account_id=abc", "account_id 无效"},
		{"/overview?start_date=2026-06-01&end_date=2026-06-02&granularity=month", "granularity 无效，仅支持 day 或 week"},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, tt.target)
		require.Equal(t, tt.message, responseMessage(t, rec.Body.Bytes()), tt.target)
	}
}

func TestBusinessAnalyticsHandler_OverviewPassesGranularity(t *testing.T) {
	svc := &stubBusinessAnalyticsService{}
	router := businessAnalyticsTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/overview?start_date=2026-06-01&end_date=2026-06-07&granularity=week", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, svc.overviewFilters, 1)
	require.Equal(t, "week", svc.overviewFilters[0].Granularity)
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

	for _, tt := range []struct {
		target  string
		message string
	}{
		{"/price-change-impact?change_date=2026-06-01", "group_id 为必填项"},
		{"/price-change-impact?group_id=bad&change_date=2026-06-01", "group_id 无效"},
		{"/price-change-impact?group_id=1", "change_date 为必填项"},
		{"/price-change-impact?group_id=1&change_date=bad", "change_date 格式无效，请使用 YYYY-MM-DD"},
		{"/price-change-impact?group_id=1&change_date=2026-06-01&days=0", "days 无效"},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, tt.target)
		require.Equal(t, tt.message, responseMessage(t, rec.Body.Bytes()), tt.target)
	}
}

func TestBusinessAnalyticsHandler_PathIDsReturnChineseBadRequestMessages(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{})

	for _, tt := range []struct {
		target  string
		message string
	}{
		{"/groups/bad/channels?start_date=2026-06-01&end_date=2026-06-02", "分组 ID 无效"},
		{"/channels/bad/groups?start_date=2026-06-01&end_date=2026-06-02", "渠道账号 ID 无效"},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, tt.target)
		require.Equal(t, tt.message, responseMessage(t, rec.Body.Bytes()), tt.target)
	}
}

func TestBusinessAnalyticsHandler_PriceChangeImpactReturnsImpact(t *testing.T) {
	changeDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{
		impact: &service.PriceChangeImpactResponse{
			GroupID:                 1,
			ChangeDate:              "2026-06-01",
			BeforeRequests:          8,
			AfterRequests:           12,
			BeforeActiveUsers:       3,
			AfterActiveUsers:        4,
			BeforeRevenue:           8,
			AfterRevenue:            12,
			RevenueDelta:            4,
			BeforeChannelCost:       5,
			AfterChannelCost:        6,
			BeforeGrossProfit:       3,
			AfterGrossProfit:        6,
			GrossProfitDelta:        3,
			BeforeProfitMargin:      float64Ptr(0.375),
			AfterProfitMargin:       float64Ptr(0.5),
			BeforeAvgRateMultiplier: float64Ptr(1.25),
			AfterAvgRateMultiplier:  float64Ptr(1.5),
			NewUsers:                2,
			LostUsers:               1,
			ChangeAt:                changeDate,
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
	require.Equal(t, float64(8), envelope.Data["before_requests"])
	require.Equal(t, float64(4), envelope.Data["after_active_users"])
	require.Equal(t, float64(6), envelope.Data["after_channel_cost"])
	require.Equal(t, float64(0.375), envelope.Data["before_profit_margin"])
	require.Equal(t, float64(1.25), envelope.Data["before_avg_rate_multiplier"])
	require.Equal(t, float64(2), envelope.Data["new_users"])
	require.Equal(t, float64(1), envelope.Data["lost_users"])
}

func TestBusinessAnalyticsHandler_ExportIncludesSnapshotAndRateMultiplierColumns(t *testing.T) {
	router := businessAnalyticsTestRouter(&stubBusinessAnalyticsService{
		records: &service.BusinessRecordsResponse{
			Items: []service.BusinessRecordRow{
				{
					CreatedAt:                   time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC),
					UserID:                      3,
					UserEmail:                   "u@example.com",
					APIKeyID:                    4,
					APIKeyName:                  "prod-key",
					GroupID:                     10,
					GroupName:                   "Team A",
					AccountID:                   20,
					AccountName:                 "Channel A",
					Model:                       "gpt-5-mini",
					Revenue:                     1.2,
					ChannelCost:                 0.7,
					GrossProfit:                 0.5,
					RateMultiplier:              float64Ptr(1.125),
					ChannelPriceSnapshot:        float64Ptr(0.875),
					ChannelPriceSnapshotMissing: false,
				},
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/export?start_date=2026-06-01&end_date=2026-06-02", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "rate_multiplier")
	require.Contains(t, body, "channel_price_snapshot")
	require.Contains(t, body, "channel_price_snapshot_missing")
	require.Contains(t, body, "1.1250000000")
	require.Contains(t, body, "0.8750000000")
}

func businessAnalyticsTestRouter(svc businessAnalyticsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewBusinessAnalyticsHandler(svc)
	router := gin.New()
	router.GET("/overview", h.GetOverview)
	router.GET("/groups/:id/channels", h.GetGroupChannels)
	router.GET("/channels/:id/groups", h.GetChannelGroups)
	router.GET("/price-change-impact", h.GetPriceChangeImpact)
	router.GET("/export", h.Export)
	return router
}

func float64Ptr(v float64) *float64 {
	return &v
}

func responseMessage(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Message
}
