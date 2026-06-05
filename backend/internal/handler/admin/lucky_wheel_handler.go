package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type luckyWheelAdminSettingsService interface {
	GetSettings(ctx context.Context) (*service.LuckyWheelSettings, error)
	UpdateSettings(ctx context.Context, req service.UpdateLuckyWheelSettingsRequest) error
}

type luckyWheelAdminGameService interface {
	ListAdminSpins(ctx context.Context, params pagination.PaginationParams, filter service.LuckyWheelAdminSpinFilter) ([]service.LuckyWheelSpinRecord, *pagination.PaginationResult, error)
}

type LuckyWheelHandler struct {
	settingsService luckyWheelAdminSettingsService
	gameService     luckyWheelAdminGameService
}

func NewLuckyWheelHandler(settingsService *service.LuckyWheelAdminService, gameService *service.LuckyWheelService) *LuckyWheelHandler {
	return &LuckyWheelHandler{
		settingsService: settingsService,
		gameService:     gameService,
	}
}

func (h *LuckyWheelHandler) GetSettings(c *gin.Context) {
	result, err := h.settingsService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *LuckyWheelHandler) UpdateSettings(c *gin.Context) {
	var req service.UpdateLuckyWheelSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingsService.UpdateSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}

func (h *LuckyWheelHandler) ListSpins(c *gin.Context) {
	userID, ok := parseOptionalLuckyWheelUserID(c)
	if !ok {
		return
	}
	startTime, endTime, ok := parseLuckyWheelDateRange(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListAdminSpins(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.LuckyWheelAdminSpinFilter{
		UserID:    userID,
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}

func parseOptionalLuckyWheelUserID(c *gin.Context) (*int64, bool) {
	raw := c.Query("user_id")
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user_id")
		return nil, false
	}
	return &parsed, true
}

func parseLuckyWheelDateRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	startRaw := c.Query("start_date")
	endRaw := c.Query("end_date")
	if startRaw == "" && endRaw == "" {
		return nil, nil, true
	}
	var (
		startTime *time.Time
		endTime   *time.Time
	)
	if startRaw != "" {
		parsed, err := time.Parse("2006-01-02", startRaw)
		if err != nil {
			response.BadRequest(c, "Invalid start_date")
			return nil, nil, false
		}
		startTime = &parsed
	}
	if endRaw != "" {
		parsed, err := time.Parse("2006-01-02", endRaw)
		if err != nil {
			response.BadRequest(c, "Invalid end_date")
			return nil, nil, false
		}
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		endTime = &parsed
	}
	return startTime, endTime, true
}
