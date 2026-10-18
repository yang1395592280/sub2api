package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCustomFeaturesZenxiangGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stored := range []map[string]string{{}, {service.SettingKeyZenxiangLiyuEnabled: "true"}, {service.SettingKeyZenxiangLiyuEnabled: "false"}} {
		settings := service.NewSettingService(&channelMonitorRouteSettingRepoStub{values: stored}, &config.Config{})
		router := gin.New()
		group := router.Group("/api/v1/zenxiang-liyu", zenxiangLiyuFeatureGuard(settings))
		calls := 0
		handle := func(c *gin.Context) { calls++; c.Status(http.StatusNoContent) }
		group.GET("/status", handle)
		group.GET("/records", handle)
		group.POST("/play", handle)
		for _, route := range []struct{ method, path string }{{"GET", "/status"}, {"GET", "/records"}, {"POST", "/play"}} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, "/api/v1/zenxiang-liyu"+route.path, nil))
			if stored[service.SettingKeyZenxiangLiyuEnabled] != "false" {
				require.Equal(t, http.StatusNoContent, w.Code)
			} else if route.path == "/status" {
				require.Equal(t, http.StatusOK, w.Code)
				require.Contains(t, w.Body.String(), `"visible":false`)
			} else {
				require.Equal(t, http.StatusNotFound, w.Code)
			}
		}
		if stored[service.SettingKeyZenxiangLiyuEnabled] == "false" {
			require.Zero(t, calls, "disabled routes must never reach the business handler")
		} else {
			require.Equal(t, 3, calls)
		}
	}
}
