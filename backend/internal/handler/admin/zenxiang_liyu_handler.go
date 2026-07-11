package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type zenxiangLiyuAdminService interface {
	GetSettings(context.Context) (*service.ZenxiangLiyuSettings, error)
	UpdateSettings(context.Context, service.ZenxiangLiyuSettingsUpdate) (*service.ZenxiangLiyuSettings, error)
	ListPrizes(context.Context) ([]service.ZenxiangLiyuPrize, error)
	SavePrize(context.Context, service.ZenxiangLiyuPrizeUpdate) (*service.ZenxiangLiyuPrize, error)
	SavePrizes(context.Context, []service.ZenxiangLiyuPrizeUpdate) ([]service.ZenxiangLiyuPrize, error)
	DeletePrize(context.Context, int64) error
	ListGrants(context.Context, int, int) ([]service.ZenxiangLiyuGrant, int, error)
	SaveGrant(context.Context, service.ZenxiangLiyuGrant) (*service.ZenxiangLiyuGrant, error)
	DeleteGrant(context.Context, int64) error
	GiftTickets(context.Context, service.ZenxiangLiyuTicketGiftRequest) (*service.ZenxiangLiyuTicketGift, error)
	GetOverviewStats(context.Context) (*service.ZenxiangLiyuOverviewStats, error)
	ListUserStats(context.Context, int, int, time.Time) ([]service.ZenxiangLiyuUserStats, int, error)
	ListPrizeStats(context.Context) ([]service.ZenxiangLiyuPrizeStats, error)
	ListPeriodStats(context.Context, string) ([]service.ZenxiangLiyuPeriodStats, error)
	ResetUserDailyPlays(context.Context, service.ZenxiangLiyuResetDailyPlayRequest) (*service.ZenxiangLiyuResetDailyPlayResult, error)
	Simulate(context.Context, service.ZenxiangLiyuSimulationRequest) (*service.ZenxiangLiyuSimulationResult, error)
	Recommend(context.Context, service.ZenxiangLiyuRecommendationRequest) (*service.ZenxiangLiyuRecommendationResult, error)
	PreviewProfit(context.Context, service.ZenxiangLiyuProfitPreviewRequest) (*service.ZenxiangLiyuProfitPreviewResult, error)
	ApplySimulation(context.Context, []service.ZenxiangLiyuPrizeUpdate) ([]service.ZenxiangLiyuPrize, error)
}

type ZenxiangLiyuHandler struct{ service zenxiangLiyuAdminService }

func NewZenxiangLiyuHandler(service zenxiangLiyuAdminService) *ZenxiangLiyuHandler {
	return &ZenxiangLiyuHandler{service: service}
}

type zenxiangLiyuPrizeListRequest struct {
	Prizes []service.ZenxiangLiyuPrizeUpdate `json:"prizes" binding:"required"`
}
type zenxiangLiyuGrantRequest struct {
	UserID  int64  `json:"user_id" binding:"required"`
	Enabled *bool  `json:"enabled"`
	Notes   string `json:"notes"`
}
type zenxiangLiyuTicketGiftRequest struct {
	RequestID   string `json:"request_id" binding:"required"`
	UserID      int64  `json:"user_id" binding:"required"`
	TicketCount int    `json:"ticket_count" binding:"required"`
	Notes       string `json:"notes"`
}

func (h *ZenxiangLiyuHandler) GetSettings(c *gin.Context) {
	data, err := h.service.GetSettings(c.Request.Context())
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) UpdateSettings(c *gin.Context) {
	var req service.ZenxiangLiyuSettingsUpdate
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.UpdateSettings(c.Request.Context(), req)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) ListPrizes(c *gin.Context) {
	data, err := h.service.ListPrizes(c.Request.Context())
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) SavePrize(c *gin.Context) {
	var req service.ZenxiangLiyuPrizeUpdate
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	if id := c.Param("id"); id != "" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid prize ID")
			return
		}
		req.ID = parsed
	}
	data, err := h.service.SavePrize(c.Request.Context(), req)
	h.respond(c, data, err)
}

// SavePrizes replaces the complete prize configuration and disables omitted tiers.
func (h *ZenxiangLiyuHandler) SavePrizes(c *gin.Context) {
	var req zenxiangLiyuPrizeListRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.SavePrizes(c.Request.Context(), req.Prizes)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) DeletePrize(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid prize ID")
		return
	}
	if err := h.service.DeletePrize(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}
func (h *ZenxiangLiyuHandler) ListGrants(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	grants, total, err := h.service.ListGrants(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, grants, int64(total), page, pageSize)
}
func (h *ZenxiangLiyuHandler) CreateGrant(c *gin.Context) {
	var req zenxiangLiyuGrantRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	grant := service.ZenxiangLiyuGrant{UserID: req.UserID, Enabled: enabled, Notes: req.Notes}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		grant.GrantedBy = &subject.UserID
	}
	data, err := h.service.SaveGrant(c.Request.Context(), grant)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) DeleteGrant(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	if err := h.service.DeleteGrant(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"user_id": userID})
}
func (h *ZenxiangLiyuHandler) GiftTickets(c *gin.Context) {
	var req zenxiangLiyuTicketGiftRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	gift := service.ZenxiangLiyuTicketGiftRequest{
		RequestID:   req.RequestID,
		UserID:      req.UserID,
		TicketCount: req.TicketCount,
		Notes:       req.Notes,
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		gift.GrantedBy = &subject.UserID
	}
	data, err := h.service.GiftTickets(c.Request.Context(), gift)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) ResetGrantDailyPlays(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	req := service.ZenxiangLiyuResetDailyPlayRequest{UserID: userID}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		req.ResetBy = &subject.UserID
	}
	data, err := h.service.ResetUserDailyPlays(c.Request.Context(), req)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) GetOverviewStats(c *gin.Context) {
	data, err := h.service.GetOverviewStats(c.Request.Context())
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) GetPeriodStats(c *gin.Context) {
	data, err := h.service.ListPeriodStats(c.Request.Context(), c.DefaultQuery("period", "day"))
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) GetUserStats(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var playDate time.Time
	if rawDate := c.Query("date"); rawDate != "" {
		parsed, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			response.BadRequest(c, "Invalid date, expected YYYY-MM-DD")
			return
		}
		playDate = parsed
	}
	stats, total, err := h.service.ListUserStats(c.Request.Context(), page, pageSize, playDate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, stats, int64(total), page, pageSize)
}
func (h *ZenxiangLiyuHandler) GetPrizeStats(c *gin.Context) {
	data, err := h.service.ListPrizeStats(c.Request.Context())
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) Simulate(c *gin.Context) {
	var req service.ZenxiangLiyuSimulationRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.Simulate(c.Request.Context(), req)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) Recommend(c *gin.Context) {
	var req service.ZenxiangLiyuRecommendationRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.Recommend(c.Request.Context(), req)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) PreviewProfit(c *gin.Context) {
	var req service.ZenxiangLiyuProfitPreviewRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.PreviewProfit(c.Request.Context(), req)
	h.respond(c, data, err)
}
func (h *ZenxiangLiyuHandler) ApplySimulation(c *gin.Context) {
	var req zenxiangLiyuPrizeListRequest
	if !bindZenxiangLiyuJSON(c, &req) {
		return
	}
	data, err := h.service.ApplySimulation(c.Request.Context(), req.Prizes)
	h.respond(c, data, err)
}

func bindZenxiangLiyuJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return false
	}
	return true
}
func (h *ZenxiangLiyuHandler) respond(c *gin.Context, data any, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
