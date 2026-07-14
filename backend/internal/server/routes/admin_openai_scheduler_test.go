package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterOpenAIAutoSchedulerRoutesExposesOverviewAndHealthBeforeMutations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1/admin")
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{OpenAIAutoScheduler: adminhandler.NewOpenAIAutoSchedulerHandler(nil, nil, nil, nil, nil)}}

	registerOpenAIAutoSchedulerRoutes(group, handlers)

	routes := router.Routes()
	indices := make(map[string]int, len(routes))
	for i, route := range routes {
		indices[route.Method+" "+route.Path] = i
	}
	overview, hasOverview := indices["GET /api/v1/admin/openai-auto-scheduler/overview"]
	health, hasHealth := indices["GET /api/v1/admin/openai-auto-scheduler/health"]
	reset, hasReset := indices["POST /api/v1/admin/openai-auto-scheduler/scores/accounts/:account_id/reset"]
	require.True(t, hasOverview)
	require.True(t, hasHealth)
	require.True(t, hasReset)
	require.Less(t, overview, reset)
	require.Less(t, health, reset)
}

func TestRegisterAdminRoutesProtectsOpenAIAutoSchedulerOverviewWithAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterAdminRoutes(
		v1,
		&handler.Handlers{Admin: &handler.AdminHandlers{}},
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }),
		nil,
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/overview", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
