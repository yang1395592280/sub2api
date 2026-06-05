package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGameCenterRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			User:             &handler.UserHandler{},
			Usage:            &handler.UsageHandler{},
			APIKey:           &handler.APIKeyHandler{},
			Totp:             &handler.TotpHandler{},
			Redeem:           &handler.RedeemHandler{},
			Subscription:     &handler.SubscriptionHandler{},
			Announcement:     &handler.AnnouncementHandler{},
			AvailableChannel: &handler.AvailableChannelHandler{},
			ChannelMonitor:   &handler.ChannelMonitorUserHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
	)

	return router
}

func TestUserFacingGameCenterRoutesAreRegistered(t *testing.T) {
	router := newGameCenterRoutesTestRouter()
	registered := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "game center overview", method: http.MethodGet, path: "/api/v1/game-center/overview"},
		{name: "game center ledger", method: http.MethodGet, path: "/api/v1/game-center/ledger"},
		{name: "usage leaderboard overview", method: http.MethodGet, path: "/api/v1/usage-leaderboard/overview"},
		{name: "usage leaderboard items", method: http.MethodGet, path: "/api/v1/usage-leaderboard/items"},
		{name: "user checkin status", method: http.MethodGet, path: "/api/v1/user/checkin"},
		{name: "user checkin claim", method: http.MethodPost, path: "/api/v1/user/checkin"},
		{name: "user checkin lucky bonus", method: http.MethodPost, path: "/api/v1/user/checkin/lucky-bonus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := registered[tt.method+" "+tt.path]
			require.True(t, ok, "route %s %s should be explicitly registered for the new user-facing game center surface", tt.method, tt.path)
		})
	}
}
