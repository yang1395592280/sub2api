package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type gameCenterService interface {
	GetOverview(ctx context.Context, userID int64, params pagination.PaginationParams) (*service.GameCenterOverview, error)
	ClaimPoints(ctx context.Context, userID int64, batchKey string) error
	ExchangeBalanceToPoints(ctx context.Context, userID int64, amount float64) (*service.GameCenterExchangeResult, error)
	ExchangePointsToBalance(ctx context.Context, userID int64, points int64) (*service.GameCenterExchangeResult, error)
	GetCatalog(ctx context.Context) ([]service.GameCatalog, error)
	GetLedger(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.GamePointsLedgerItem, *pagination.PaginationResult, error)
}

type GameCenterHandler struct {
	service gameCenterService
}

type exchangeBalanceToPointsRequest struct {
	Amount float64 `json:"amount" binding:"required"`
}

type exchangePointsToBalanceRequest struct {
	Points int64 `json:"points" binding:"required"`
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
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) ClaimPoints(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.service.ClaimPoints(c.Request.Context(), subject.UserID, c.Param("batchKey")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "claimed"})
}

func (h *GameCenterHandler) ExchangeBalanceToPoints(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req exchangeBalanceToPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.ExchangeBalanceToPoints(c.Request.Context(), subject.UserID, req.Amount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) ExchangePointsToBalance(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req exchangePointsToBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.ExchangePointsToBalance(c.Request.Context(), subject.UserID, req.Points)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *GameCenterHandler) GetCatalog(c *gin.Context) {
	result, err := h.service.GetCatalog(c.Request.Context())
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
	items, result, err := h.service.GetLedger(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result == nil {
		response.Success(c, items)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}
