package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
