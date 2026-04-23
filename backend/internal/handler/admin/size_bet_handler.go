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

type sizeBetSettingsService interface {
	GetSettings(ctx context.Context) (*service.SizeBetSettings, error)
	UpdateSettings(ctx context.Context, req service.UpdateSizeBetSettingsRequest) error
}

type sizeBetAdminGameService interface {
	ListRounds(ctx context.Context, params pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error)
	ListBets(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminBetFilter) ([]service.SizeBetAdminBet, *pagination.PaginationResult, error)
	ListLedger(ctx context.Context, params pagination.PaginationParams, filter service.SizeBetAdminLedgerFilter) ([]service.SizeBetLedgerEntry, *pagination.PaginationResult, error)
	RefundRound(ctx context.Context, roundID int64, refundedAt time.Time) (*service.SizeBetRefundResult, error)
}

type SizeBetHandler struct {
	settingsService sizeBetSettingsService
	gameService     sizeBetAdminGameService
}

func NewSizeBetHandler(settingsService *service.SizeBetAdminService, gameService *service.SizeBetService) *SizeBetHandler {
	return &SizeBetHandler{
		settingsService: settingsService,
		gameService:     gameService,
	}
}

func (h *SizeBetHandler) GetSettings(c *gin.Context) {
	result, err := h.settingsService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *SizeBetHandler) UpdateSettings(c *gin.Context) {
	var req service.UpdateSizeBetSettingsRequest
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

func (h *SizeBetHandler) ListRounds(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListRounds(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, items, paginationResult)
}

func (h *SizeBetHandler) ListBets(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListBets(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.SizeBetAdminBetFilter{
		RoundID: parseOptionalInt64Query(c, "round_id"),
		UserID:  parseOptionalInt64Query(c, "user_id"),
		Status:  c.Query("status"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, items, paginationResult)
}

func (h *SizeBetHandler) ListLedger(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.gameService.ListLedger(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.SizeBetAdminLedgerFilter{
		RoundID:   parseOptionalInt64Query(c, "round_id"),
		UserID:    parseOptionalInt64Query(c, "user_id"),
		EntryType: c.Query("entry_type"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	writePaginated(c, items, paginationResult)
}

func (h *SizeBetHandler) RefundRound(c *gin.Context) {
	roundID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	result, err := h.gameService.RefundRound(c.Request.Context(), roundID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func writePaginated(c *gin.Context, items any, paginationResult *pagination.PaginationResult) {
	response.PaginatedWithResult(c, items, &response.PaginationResult{
		Total:    paginationResult.Total,
		Page:     paginationResult.Page,
		PageSize: paginationResult.PageSize,
		Pages:    paginationResult.Pages,
	})
}

func parseOptionalInt64Query(c *gin.Context, key string) *int64 {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}
