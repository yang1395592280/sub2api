package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type gameCenterAdminService interface {
	GetAdminSettings(ctx context.Context) (*service.GameCenterAdminSettings, error)
	UpdateAdminSettings(ctx context.Context, req service.GameCenterAdminSettings) error
	GetCatalog(ctx context.Context) ([]service.GameCatalog, error)
	UpdateCatalog(ctx context.Context, gameKey string, req service.UpdateGameCatalogRequest) error
	GetAdminLedger(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GameCenterAdminLedgerItem, *pagination.PaginationResult, error)
	GetClaimRecords(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GameCenterClaimRecord, *pagination.PaginationResult, error)
	GetExchangeRecords(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GameCenterExchangeRecord, *pagination.PaginationResult, error)
	AdjustPoints(ctx context.Context, input service.AdminAdjustPointsInput) error
}

type GameCenterHandler struct {
	service gameCenterAdminService
}

type adjustPointsRequest struct {
	DeltaPoints int64  `json:"delta_points" binding:"required"`
	Reason      string `json:"reason"`
}

func NewGameCenterHandler(gameCenterService *service.GameCenterService) *GameCenterHandler {
	return &GameCenterHandler{service: gameCenterService}
}

func (h *GameCenterHandler) GetSettings(c *gin.Context) {
	result, err := h.service.GetAdminSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) UpdateSettings(c *gin.Context) {
	var req service.GameCenterAdminSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.UpdateAdminSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *GameCenterHandler) ListCatalog(c *gin.Context) {
	result, err := h.service.GetCatalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) UpdateCatalog(c *gin.Context) {
	var req service.UpdateGameCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.UpdateCatalog(c.Request.Context(), c.Param("gameKey"), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *GameCenterHandler) ListLedger(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter, ok := parseAdminGameCenterFilter(c)
	if !ok {
		return
	}
	items, result, err := h.service.GetAdminLedger(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *GameCenterHandler) ListClaims(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter, ok := parseAdminGameCenterFilter(c)
	if !ok {
		return
	}
	items, result, err := h.service.GetClaimRecords(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *GameCenterHandler) ListExchanges(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter, ok := parseAdminGameCenterFilter(c)
	if !ok {
		return
	}
	items, result, err := h.service.GetExchangeRecords(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func parseAdminGameCenterFilter(c *gin.Context) (service.GamePointsLedgerFilter, bool) {
	filter := service.GamePointsLedgerFilter{}
	userID, err := parseOptionalUserID(c.Query("user_id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return filter, false
	}
	filter.UserID = userID
	start, end, err := parseAdminGameCenterTimeRange(c.Query("start_time"), c.Query("end_time"), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return filter, false
	}
	filter.StartTime = start
	filter.EndTime = end
	return filter, true
}

func (h *GameCenterHandler) AdjustPoints(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	var req adjustPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.AdjustPoints(c.Request.Context(), service.AdminAdjustPointsInput{
		UserID:      userID,
		DeltaPoints: req.DeltaPoints,
		Reason:      strings.TrimSpace(req.Reason),
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func parseOptionalUserID(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, errors.New("invalid user_id")
	}
	return &value, nil
}

func parseAdminGameCenterTimeRange(startTimeRaw, endTimeRaw, startDateRaw, endDateRaw string) (*time.Time, *time.Time, error) {
	parseDate := func(raw string, endOfDay bool) (*time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, nil
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return &t, nil
		}
		t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			return nil, err
		}
		if endOfDay {
			t = t.AddDate(0, 0, 1)
		}
		return &t, nil
	}
	start, err := parseDate(firstAdminNonEmpty(startTimeRaw, startDateRaw), false)
	if err != nil {
		return nil, nil, err
	}
	end, err := parseDate(firstAdminNonEmpty(endTimeRaw, endDateRaw), true)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

func firstAdminNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
