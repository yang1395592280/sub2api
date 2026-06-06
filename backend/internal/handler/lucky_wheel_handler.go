package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type luckyWheelService interface {
	GetOverview(ctx context.Context, userID int64) (*service.LuckyWheelOverview, error)
	Spin(ctx context.Context, userID int64) (*service.LuckyWheelSpinResult, error)
	GetHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.LuckyWheelSpinRecord, *pagination.PaginationResult, error)
	GetLeaderboard(ctx context.Context) (*service.LuckyWheelLeaderboardView, error)
}

type LuckyWheelHandler struct {
	service luckyWheelService
}

func NewLuckyWheelHandler(luckyWheelService *service.LuckyWheelService) *LuckyWheelHandler {
	return &LuckyWheelHandler{service: luckyWheelService}
}

func (h *LuckyWheelHandler) GetOverview(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.GetOverview(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *LuckyWheelHandler) Spin(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.Spin(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *LuckyWheelHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.service.GetHistory(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(paginationResult))
}

func (h *LuckyWheelHandler) GetLeaderboard(c *gin.Context) {
	result, err := h.service.GetLeaderboard(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func toResponsePagination(result *pagination.PaginationResult) *response.PaginationResult {
	if result == nil {
		return nil
	}
	return &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	}
}
