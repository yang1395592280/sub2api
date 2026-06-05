package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type gameCenterService interface {
	GetOverview(ctx context.Context, userID int64, params pagination.PaginationParams, userTZ string) (*service.GameCenterOverview, error)
	GetLedger(ctx context.Context, userID int64, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GamePointsLedgerItem, *pagination.PaginationResult, error)
	GetUserLedger(ctx context.Context, userID int64, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GamePointsLedgerItem, *pagination.PaginationResult, error)
	GetPointsLeaderboard(ctx context.Context, params pagination.PaginationParams) ([]service.GamePointsLeaderboardItem, *pagination.PaginationResult, error)
}

type GameCenterHandler struct {
	service gameCenterService
}

func NewGameCenterHandler(svc *service.GameCenterService) *GameCenterHandler {
	return &GameCenterHandler{service: svc}
}

func (h *GameCenterHandler) GetOverview(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.GetOverview(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) GetLedger(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	filter, ok := parseGameCenterTimeFilter(c)
	if !ok {
		return
	}
	items, result, err := h.service.GetLedger(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}

func (h *GameCenterHandler) GetPointsLeaderboard(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.GetPointsLeaderboard(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}

func (h *GameCenterHandler) GetUserLedger(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin && userID != subject.UserID {
		response.ErrorFrom(c, infraerrors.Forbidden("GAME_CENTER_LEDGER_FORBIDDEN", "cannot view another user's points ledger"))
		return
	}
	filter, ok := parseGameCenterTimeFilter(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.GetUserLedger(c.Request.Context(), userID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}

func parseGameCenterTimeFilter(c *gin.Context) (service.GamePointsLedgerFilter, bool) {
	filter := service.GamePointsLedgerFilter{}
	start, end, err := parseGameCenterTimeRange(c.Query("start_time"), c.Query("end_time"), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return filter, false
	}
	filter.StartTime = start
	filter.EndTime = end
	return filter, true
}

func parseGameCenterTimeRange(startTimeRaw, endTimeRaw, startDateRaw, endDateRaw string) (*time.Time, *time.Time, error) {
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
	start, err := parseDate(firstGameCenterNonEmpty(startTimeRaw, startDateRaw), false)
	if err != nil {
		return nil, nil, err
	}
	end, err := parseDate(firstGameCenterNonEmpty(endTimeRaw, endDateRaw), true)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

func firstGameCenterNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
