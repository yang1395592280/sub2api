package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAISchedulerHandlerStatsRepoStub struct {
	stats *service.OpenAISchedulerDailyStats
}

type openAISchedulerHandlerRoutingRepoStub struct {
	service.AccountRepository
	accounts []service.Account
	getByID  func(context.Context, int64) (*service.Account, error)
	listErr  error
}

func (r openAISchedulerHandlerRoutingRepoStub) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	if r.getByID != nil {
		return r.getByID(ctx, id)
	}
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, service.ErrAccountNotFound
}

func (r openAISchedulerHandlerRoutingRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listByPlatform(platform), nil
}

func (r openAISchedulerHandlerRoutingRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listByPlatform(platform), nil
}

func (r openAISchedulerHandlerRoutingRepoStub) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.listByPlatform(platform), nil
}

func (r openAISchedulerHandlerRoutingRepoStub) listByPlatform(platform string) []service.Account {
	result := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result
}

func (r *openAISchedulerHandlerStatsRepoStub) IncrementDailySelection(ctx context.Context, statDate time.Time, groupID int64, accountID int64, selectedAt time.Time) error {
	return nil
}

func (r *openAISchedulerHandlerStatsRepoStub) GetDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*service.OpenAISchedulerDailyStats, error) {
	if r.stats != nil {
		return r.stats, nil
	}
	return &service.OpenAISchedulerDailyStats{Date: statDate.Format("2006-01-02"), GroupID: groupID}, nil
}

func (r *openAISchedulerHandlerStatsRepoStub) RecomputeDailyStatsFromUsageLogs(ctx context.Context, statDate time.Time, start time.Time, end time.Time, groupID int64) (*service.OpenAISchedulerDailyStats, error) {
	return r.GetDailyStats(ctx, statDate, groupID)
}

func TestOpenAISchedulerHandler_GetSettings_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/settings", h.GetSettings)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"health_ranking_enabled":false`)
	require.Contains(t, w.Body.String(), `"primary_ratio":0.3`)
}

func TestOpenAISchedulerHandler_ListAccounts_NoAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/accounts", h.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/accounts?page=1&page_size=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items":[]`)
	require.Contains(t, w.Body.String(), `"total":0`)
}

func TestOpenAISchedulerHandler_ActionInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.POST("/accounts/:id/actions", h.ApplyAction)

	req := httptest.NewRequest(http.MethodPost, "/accounts/bad/actions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpenAISchedulerHandler_ListAccounts_ResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/accounts", h.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)
	require.Contains(t, w.Body.String(), `"page"`)
	require.Contains(t, w.Body.String(), `"page_size"`)
	require.Contains(t, w.Body.String(), `"total"`)
}

func TestOpenAISchedulerHandler_GetOverview_IncludesTierCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/overview", h.GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"tier_counts"`)
	require.Contains(t, w.Body.String(), `"primary":0`)
	require.Contains(t, w.Body.String(), `"standby":0`)
	require.Contains(t, w.Body.String(), `"observe":0`)
	require.Contains(t, w.Body.String(), `"degraded":0`)
}

func TestOpenAISchedulerHandler_GetOverview_InvalidGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/overview", h.GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/overview?group_id=bad", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpenAISchedulerHandler_GetDailyStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	statsRepo := &openAISchedulerHandlerStatsRepoStub{
		stats: &service.OpenAISchedulerDailyStats{
			Date:         "2026-06-13",
			GroupID:      33,
			TotalSelects: 10,
			Accounts: []service.OpenAISchedulerAccountDailyStat{
				{AccountID: 11855, SelectCount: 7, SelectRatio: 0.7},
				{AccountID: 11845, SelectCount: 3, SelectRatio: 0.3},
			},
		},
	}
	h := NewOpenAISchedulerHandler(service.NewOpenAIGatewayService(nil, nil, nil, statsRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	router := gin.New()
	router.GET("/stats", h.GetDailyStats)

	req := httptest.NewRequest(http.MethodGet, "/stats?group_id=33&date=2026-06-13", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"total_selects":10`)
	require.Contains(t, w.Body.String(), `"account_id":11855`)
	require.Contains(t, w.Body.String(), `"select_ratio":0.7`)
}

func TestOpenAISchedulerHandler_GetDailyStats_InvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/stats", h.GetDailyStats)

	req := httptest.NewRequest(http.MethodGet, "/stats?group_id=33&date=bad", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpenAISchedulerHandler_RoutingRanking_ResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	r := gin.New()
	r.GET("/ranking", h.GetRoutingRanking)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ranking?model=gpt-5.1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)
	require.Contains(t, w.Body.String(), `"snapshot_at"`)
}

func TestOpenAISchedulerHandler_RoutingExplain_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	r := gin.New()
	r.GET("/accounts/:id/routing-explain", h.GetRoutingExplain)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/bad/routing-explain", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpenAISchedulerHandler_RoutingPlatformQueryDefaultsOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/ranking?platform=+++", nil)

	require.Equal(t, service.PlatformOpenAI, openAIRoutingPlatformQuery(c))
}

func TestOpenAISchedulerHandler_RoutingRanking_BlankPlatformDefaultsToOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(service.NewOpenAIGatewayService(
		openAISchedulerHandlerRoutingRepoStub{
			accounts: []service.Account{
				{ID: 123, Name: "openai-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1, Concurrency: 1},
			},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	))
	r := gin.New()
	r.GET("/ranking", h.GetRoutingRanking)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ranking?platform=+++", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"account_id":123`)
}

func TestOpenAISchedulerHandler_RoutingExplain_AccountNotFoundReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(service.NewOpenAIGatewayService(
		openAISchedulerHandlerRoutingRepoStub{
			getByID: func(context.Context, int64) (*service.Account, error) {
				return nil, service.ErrAccountNotFound
			},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	))
	r := gin.New()
	r.GET("/accounts/:id/routing-explain", h.GetRoutingExplain)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/404/routing-explain", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "routing explanation not found")
}

func TestOpenAISchedulerHandler_RoutingExplain_OtherErrorsReturn500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(service.NewOpenAIGatewayService(
		openAISchedulerHandlerRoutingRepoStub{
			listErr: errors.New("boom"),
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	))
	r := gin.New()
	r.GET("/accounts/:id/routing-explain", h.GetRoutingExplain)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/500/routing-explain", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "boom")
}
