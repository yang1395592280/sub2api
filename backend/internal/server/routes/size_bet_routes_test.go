package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSizeBetRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")

	handlers := &handler.Handlers{
		SizeBet: &handler.SizeBetHandler{},
		Admin: &handler.AdminHandlers{
			SizeBet: &adminhandler.SizeBetHandler{},
		},
	}

	RegisterUserRoutes(v1, handlers, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Next()
	}), nil)
	RegisterAdminRoutes(v1, handlers, middleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Next()
	}))

	routes := router.Routes()

	expected := map[string]struct{}{
		"GET /api/v1/game/size-bet/current":                   {},
		"POST /api/v1/game/size-bet/bet":                      {},
		"GET /api/v1/game/size-bet/history":                   {},
		"GET /api/v1/game/size-bet/rounds":                    {},
		"GET /api/v1/game/size-bet/leaderboard":               {},
		"GET /api/v1/game/size-bet/rules":                     {},
		"GET /api/v1/admin/games/size-bet/settings":           {},
		"PUT /api/v1/admin/games/size-bet/settings":           {},
		"GET /api/v1/admin/games/size-bet/rounds":             {},
		"GET /api/v1/admin/games/size-bet/bets":               {},
		"GET /api/v1/admin/games/size-bet/ledger":             {},
		"POST /api/v1/admin/games/size-bet/rounds/:id/refund": {},
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected)
}
