package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.GET("/balance-history", h.User.GetBalanceHistory)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
			}

			user.GET("/checkin", h.User.GetCheckinStatus)
			user.POST("/checkin", h.User.DoCheckin)
			user.POST("/checkin/lucky-bonus", h.User.PlayCheckinLuckyBonus)
		}

		windsurfAccounts := authenticated.Group("/windsurf-accounts")
		{
			windsurfAccounts.GET("", h.WindsurfAccount.List)
			windsurfAccounts.POST("", h.WindsurfAccount.Create)
			windsurfAccounts.PUT("/:id", h.WindsurfAccount.UpdateCredentials)
			windsurfAccounts.PUT("/:id/status", h.WindsurfAccount.UpdateStatus)
			windsurfAccounts.DELETE("/:id", h.WindsurfAccount.Delete)
			windsurfAccounts.GET("/:id/password", h.WindsurfAccount.RevealPassword)
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		game := authenticated.Group("/game/size-bet")
		{
			game.GET("/current", h.SizeBet.GetCurrent)
			game.POST("/bet", h.SizeBet.PlaceBet)
			game.GET("/history", h.SizeBet.GetHistory)
			game.GET("/rounds", h.SizeBet.ListRecentRounds)
			game.GET("/stats/overview", h.SizeBet.GetStatsOverview)
			game.GET("/stats/users", h.SizeBet.ListStatsUsers)
			game.GET("/leaderboard", h.SizeBet.GetLeaderboard)
			game.GET("/rules", h.SizeBet.GetRules)
		}

		luckyWheel := authenticated.Group("/game/lucky-wheel")
		{
			luckyWheel.GET("/overview", h.LuckyWheel.GetOverview)
			luckyWheel.POST("/spin", h.LuckyWheel.Spin)
			luckyWheel.GET("/history", h.LuckyWheel.GetHistory)
			luckyWheel.GET("/leaderboard", h.LuckyWheel.GetLeaderboard)
		}

		gameCenter := authenticated.Group("/game-center")
		{
			gameCenter.GET("/overview", h.GameCenter.GetOverview)
			gameCenter.POST("/claims/:batchKey", h.GameCenter.ClaimPoints)
			gameCenter.POST("/exchange/balance-to-points", h.GameCenter.ExchangeBalanceToPoints)
			gameCenter.POST("/exchange/points-to-balance", h.GameCenter.ExchangePointsToBalance)
			gameCenter.GET("/ledger", h.GameCenter.GetLedger)
			gameCenter.GET("/leaderboard", h.GameCenter.GetPointsLeaderboard)
			gameCenter.GET("/users/:id/ledger", h.GameCenter.GetUserLedger)
			gameCenter.GET("/catalog", h.GameCenter.GetCatalog)
		}
	}
}
