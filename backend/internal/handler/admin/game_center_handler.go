package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"

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
	GetAdminLedger(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterAdminLedgerItem, *pagination.PaginationResult, error)
	GetClaimRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterClaimRecord, *pagination.PaginationResult, error)
	GetExchangeRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterExchangeRecord, *pagination.PaginationResult, error)
	AdjustPoints(ctx context.Context, input service.AdminAdjustPointsInput) error
}

type GameCenterHandler struct {
	service gameCenterAdminService
}

type adjustPointsRequest struct {
	DeltaPoints int64  `json:"delta_points" binding:"required"`
	Reason      string `json:"reason" binding:"required"`
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
	userID, err := parseOptionalUserID(c.Query("user_id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, result, err := h.service.GetAdminLedger(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *GameCenterHandler) ListClaims(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	userID, err := parseOptionalUserID(c.Query("user_id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, result, err := h.service.GetClaimRecords(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *GameCenterHandler) ListExchanges(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	userID, err := parseOptionalUserID(c.Query("user_id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, result, err := h.service.GetExchangeRecords(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
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
