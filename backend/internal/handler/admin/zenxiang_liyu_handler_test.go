package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubAdminZenxiangLiyuService struct {
	lastSettings service.ZenxiangLiyuSettingsUpdate
	simulation   *service.ZenxiangLiyuSimulationResult
	bulkPrizes   []service.ZenxiangLiyuPrizeUpdate
	applyPrizes  []service.ZenxiangLiyuPrizeUpdate
	lastGift     service.ZenxiangLiyuTicketGiftRequest
}

func (s *stubAdminZenxiangLiyuService) GetSettings(context.Context) (*service.ZenxiangLiyuSettings, error) {
	return nil, nil
}

func (s *stubAdminZenxiangLiyuService) UpdateSettings(_ context.Context, settings service.ZenxiangLiyuSettingsUpdate) (*service.ZenxiangLiyuSettings, error) {
	s.lastSettings = settings
	return &service.ZenxiangLiyuSettings{}, nil
}

func (s *stubAdminZenxiangLiyuService) Simulate(context.Context, service.ZenxiangLiyuSimulationRequest) (*service.ZenxiangLiyuSimulationResult, error) {
	return s.simulation, nil
}

func (s *stubAdminZenxiangLiyuService) ListPrizes(context.Context) ([]service.ZenxiangLiyuPrize, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) SavePrize(context.Context, service.ZenxiangLiyuPrizeUpdate) (*service.ZenxiangLiyuPrize, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) SavePrizes(_ context.Context, prizes []service.ZenxiangLiyuPrizeUpdate) ([]service.ZenxiangLiyuPrize, error) {
	s.bulkPrizes = prizes
	return []service.ZenxiangLiyuPrize{{ID: 1, Name: prizes[0].Name}}, nil
}
func (s *stubAdminZenxiangLiyuService) DeletePrize(context.Context, int64) error { return nil }
func (s *stubAdminZenxiangLiyuService) ListGrants(context.Context, int, int) ([]service.ZenxiangLiyuGrant, int, error) {
	return nil, 0, nil
}
func (s *stubAdminZenxiangLiyuService) SaveGrant(context.Context, service.ZenxiangLiyuGrant) (*service.ZenxiangLiyuGrant, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) DeleteGrant(context.Context, int64) error { return nil }
func (s *stubAdminZenxiangLiyuService) GiftTickets(_ context.Context, gift service.ZenxiangLiyuTicketGiftRequest) (*service.ZenxiangLiyuTicketGift, error) {
	s.lastGift = gift
	return &service.ZenxiangLiyuTicketGift{RequestID: gift.RequestID, UserID: gift.UserID, TicketCount: gift.TicketCount, Notes: gift.Notes}, nil
}
func (s *stubAdminZenxiangLiyuService) GetOverviewStats(context.Context) (*service.ZenxiangLiyuOverviewStats, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) ListUserStats(context.Context, int, int, time.Time) ([]service.ZenxiangLiyuUserStats, int, error) {
	return nil, 0, nil
}
func (s *stubAdminZenxiangLiyuService) ListPrizeStats(context.Context) ([]service.ZenxiangLiyuPrizeStats, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) ListPeriodStats(context.Context, string) ([]service.ZenxiangLiyuPeriodStats, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) ResetUserDailyPlays(context.Context, service.ZenxiangLiyuResetDailyPlayRequest) (*service.ZenxiangLiyuResetDailyPlayResult, error) {
	return &service.ZenxiangLiyuResetDailyPlayResult{}, nil
}
func (s *stubAdminZenxiangLiyuService) Recommend(context.Context, service.ZenxiangLiyuRecommendationRequest) (*service.ZenxiangLiyuRecommendationResult, error) {
	return nil, nil
}
func (s *stubAdminZenxiangLiyuService) PreviewProfit(context.Context, service.ZenxiangLiyuProfitPreviewRequest) (*service.ZenxiangLiyuProfitPreviewResult, error) {
	return &service.ZenxiangLiyuProfitPreviewResult{}, nil
}
func (s *stubAdminZenxiangLiyuService) ApplySimulation(_ context.Context, prizes []service.ZenxiangLiyuPrizeUpdate) ([]service.ZenxiangLiyuPrize, error) {
	s.applyPrizes = prizes
	return []service.ZenxiangLiyuPrize{{ID: 1, Name: prizes[0].Name}}, nil
}

func TestAdminZenxiangLiyuUpdateSettingsMapsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubAdminZenxiangLiyuService{}
	h := NewZenxiangLiyuHandler(svc)
	router := gin.New()
	router.PUT("/admin/zenxiang-liyu/settings", h.UpdateSettings)
	body := `{"global_enabled":true,"ticket_amount":2,"minimum_balance":10,"daily_play_limit":5}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/admin/zenxiang-liyu/settings", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, svc.lastSettings.GlobalEnabled)
	require.Equal(t, 2.0, svc.lastSettings.TicketAmount)
}

func TestAdminZenxiangLiyuSimulateReturnsResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubAdminZenxiangLiyuService{simulation: &service.ZenxiangLiyuSimulationResult{TotalPlays: 10, NetProfit: 2}}
	h := NewZenxiangLiyuHandler(svc)
	router := gin.New()
	router.POST("/admin/zenxiang-liyu/simulate", h.Simulate)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/zenxiang-liyu/simulate", strings.NewReader(`{"user_count":1,"plays_per_user":10,"ticket_amount":2,"minimum_balance":10,"daily_play_limit":5,"prizes":[{"name":"奖项","reward_amount":1,"probability":100,"enabled":true}]}`)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"total_plays":10`)
}

func TestAdminZenxiangLiyuSavePrizesUsesBulkPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubAdminZenxiangLiyuService{}
	router := gin.New()
	router.PUT("/admin/zenxiang-liyu/prizes", NewZenxiangLiyuHandler(svc).SavePrizes)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/admin/zenxiang-liyu/prizes", strings.NewReader(`{"prizes":[{"name":"一等","reward_amount":5,"probability":100,"enabled":true}]}`)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.bulkPrizes, 1)
	require.Equal(t, "一等", svc.bulkPrizes[0].Name)
}

func TestAdminZenxiangLiyuApplySimulationUsesApplyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubAdminZenxiangLiyuService{}
	router := gin.New()
	router.POST("/admin/zenxiang-liyu/simulate/apply", NewZenxiangLiyuHandler(svc).ApplySimulation)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/zenxiang-liyu/simulate/apply", strings.NewReader(`{"prizes":[{"name":"推荐奖项","reward_amount":1,"probability":100,"enabled":true}]}`)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.applyPrizes, 1)
	require.Empty(t, svc.bulkPrizes)
}

func TestAdminZenxiangLiyuGiftTicketsMapsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubAdminZenxiangLiyuService{}
	router := gin.New()
	router.POST("/admin/zenxiang-liyu/tickets/gift", NewZenxiangLiyuHandler(svc).GiftTickets)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/zenxiang-liyu/tickets/gift", strings.NewReader(`{"request_id":"gift-1","user_id":7,"ticket_count":2,"notes":"客服补偿"}`)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "gift-1", svc.lastGift.RequestID)
	require.EqualValues(t, 7, svc.lastGift.UserID)
	require.Equal(t, 2, svc.lastGift.TicketCount)
	require.Equal(t, "客服补偿", svc.lastGift.Notes)
}
