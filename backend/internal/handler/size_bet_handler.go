package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type sizeBetService interface {
	GetCurrentRoundView(ctx context.Context, userID int64, now time.Time) (*service.SizeBetCurrentRoundView, error)
	PlaceBet(ctx context.Context, req service.PlaceSizeBetRequest) (*service.SizeBet, error)
	GetHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.SizeBetUserHistoryItem, *pagination.PaginationResult, error)
	ListRecentRounds(ctx context.Context, limit int) ([]service.SizeBetRound, error)
	GetLeaderboard(ctx context.Context, scope string, now time.Time) (*service.SizeBetLeaderboardView, error)
	GetRules(ctx context.Context, now time.Time) (*service.SizeBetRulesView, error)
}

type SizeBetHandler struct {
	service sizeBetService
}

type PlaceBetRequest struct {
	RoundID        int64                    `json:"round_id" binding:"required"`
	Direction      service.SizeBetDirection `json:"direction" binding:"required"`
	StakeAmount    float64                  `json:"stake_amount" binding:"required"`
	IdempotencyKey string                   `json:"idempotency_key"`
}

func NewSizeBetHandler(sizeBetService *service.SizeBetService) *SizeBetHandler {
	return &SizeBetHandler{service: sizeBetService}
}

func (h *SizeBetHandler) GetCurrent(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.GetCurrentRoundView(c.Request.Context(), subject.UserID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) PlaceBet(c *gin.Context) {
	var req PlaceBetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	bet, err := h.service.PlaceBet(c.Request.Context(), service.PlaceSizeBetRequest{
		UserID:         subject.UserID,
		RoundID:        req.RoundID,
		Direction:      req.Direction,
		StakeAmount:    req.StakeAmount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, bet)
}

func (h *SizeBetHandler) GetHistory(c *gin.Context) {
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
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}

func (h *SizeBetHandler) ListRecentRounds(c *gin.Context) {
	limit := 10
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.service.ListRecentRounds(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *SizeBetHandler) GetLeaderboard(c *gin.Context) {
	result, err := h.service.GetLeaderboard(c.Request.Context(), c.Query("scope"), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) GetRules(c *gin.Context) {
	result, err := h.service.GetRules(c.Request.Context(), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
