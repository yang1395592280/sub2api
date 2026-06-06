package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type usageLeaderboardService interface {
	GetOverview(ctx context.Context, userID int64, query service.UsageLeaderboardQuery) (*service.UsageLeaderboardOverview, error)
	GetItems(ctx context.Context, userID int64, query service.UsageLeaderboardQuery, params pagination.PaginationParams) ([]service.UsageLeaderboardItem, *pagination.PaginationResult, error)
}

type UsageLeaderboardHandler struct {
	service usageLeaderboardService
}

func NewUsageLeaderboardHandler(svc *service.UsageLeaderboardService) *UsageLeaderboardHandler {
	return &UsageLeaderboardHandler{service: svc}
}

func (h *UsageLeaderboardHandler) GetOverview(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	result, err := h.service.GetOverview(c.Request.Context(), subject.UserID, service.UsageLeaderboardQuery{
		Date:   c.Query("date"),
		Metric: c.Query("metric"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UsageLeaderboardHandler) GetItems(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.GetItems(c.Request.Context(), subject.UserID, service.UsageLeaderboardQuery{
		Date:   c.Query("date"),
		Metric: c.Query("metric"),
	}, pagination.PaginationParams{
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
