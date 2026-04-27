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
		SizeBet:    &handler.SizeBetHandler{},
		LuckyWheel: &handler.LuckyWheelHandler{},
		GameCenter: &handler.GameCenterHandler{},
		Admin: &handler.AdminHandlers{
			SizeBet:    &adminhandler.SizeBetHandler{},
			LuckyWheel: &adminhandler.LuckyWheelHandler{},
			GameCenter: &adminhandler.GameCenterHandler{},
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
		"GET /api/v1/game/size-bet/current":                      {},
		"POST /api/v1/game/size-bet/bet":                         {},
		"GET /api/v1/game/size-bet/history":                      {},
		"GET /api/v1/game/size-bet/rounds":                       {},
		"GET /api/v1/game/size-bet/stats/overview":               {},
		"GET /api/v1/game/size-bet/stats/users":                  {},
		"GET /api/v1/game/size-bet/leaderboard":                  {},
		"GET /api/v1/game/size-bet/rules":                        {},
		"GET /api/v1/game/lucky-wheel/overview":                  {},
		"POST /api/v1/game/lucky-wheel/spin":                     {},
		"GET /api/v1/game/lucky-wheel/history":                   {},
		"GET /api/v1/game/lucky-wheel/leaderboard":               {},
		"GET /api/v1/game-center/overview":                       {},
		"POST /api/v1/game-center/claims/:batchKey":              {},
		"POST /api/v1/game-center/exchange/balance-to-points":    {},
		"POST /api/v1/game-center/exchange/points-to-balance":    {},
		"GET /api/v1/game-center/ledger":                         {},
		"GET /api/v1/game-center/leaderboard":                    {},
		"GET /api/v1/game-center/users/:id/ledger":               {},
		"GET /api/v1/game-center/catalog":                        {},
		"GET /api/v1/admin/game-center/settings":                 {},
		"PUT /api/v1/admin/game-center/settings":                 {},
		"GET /api/v1/admin/game-center/catalog":                  {},
		"PUT /api/v1/admin/game-center/catalog/:gameKey":         {},
		"GET /api/v1/admin/game-center/ledger":                   {},
		"GET /api/v1/admin/game-center/claims":                   {},
		"GET /api/v1/admin/game-center/exchanges":                {},
		"POST /api/v1/admin/game-center/users/:id/points/adjust": {},
		"GET /api/v1/admin/games/size-bet/settings":              {},
		"PUT /api/v1/admin/games/size-bet/settings":              {},
		"GET /api/v1/admin/games/size-bet/rounds":                {},
		"GET /api/v1/admin/games/size-bet/bets":                  {},
		"GET /api/v1/admin/games/size-bet/ledger":                {},
		"GET /api/v1/admin/games/size-bet/stats/overview":        {},
		"GET /api/v1/admin/games/size-bet/stats/users":           {},
		"POST /api/v1/admin/games/size-bet/rounds/:id/refund":    {},
		"GET /api/v1/admin/games/lucky-wheel/settings":           {},
		"PUT /api/v1/admin/games/lucky-wheel/settings":           {},
		"GET /api/v1/admin/games/lucky-wheel/spins":              {},
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		delete(expected, key)
	}

	require.Empty(t, expected)
}
