package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerRoutes_RoutingExplainPathIsNotShadowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	registerOpenAISchedulerRoutes(admin, &handler.Handlers{
		Admin: &handler.AdminHandlers{
			OpenAIScheduler: adminhandler.NewOpenAISchedulerHandler(&service.OpenAIGatewayService{}),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-scheduler/accounts/123/routing-explain", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "routing explanation not found")
	require.NotContains(t, w.Body.String(), "scheduler health snapshot not found")
}
