package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *RedeemHandler) ListCampaigns(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	if page-1 > int(^uint(0)>>1)/pageSize {
		response.BadRequest(c, "invalid page")
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len([]rune(search)) > 100 {
		response.BadRequest(c, "search must contain at most 100 characters")
		return
	}
	campaigns, total, err := h.redeemService.ListCampaigns(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize}, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, campaigns, total, page, pageSize)
}

func (h *RedeemHandler) GetCampaign(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign ID")
		return
	}
	campaign, err := h.redeemService.GetCampaign(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, campaign)
}

func (h *RedeemHandler) CreateCampaign(c *gin.Context) {
	var req service.RedeemCampaignInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.redeem_campaigns.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.redeemService.CreateCampaign(ctx, req)
	})
}

func (h *RedeemHandler) UpdateCampaign(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid campaign ID")
		return
	}
	var req service.RedeemCampaignInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	executeAdminIdempotentJSON(c, "admin.redeem_campaigns.update", struct {
		ID int64 `json:"id"`
		service.RedeemCampaignInput
	}{id, req}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.redeemService.UpdateCampaign(ctx, id, req)
	})
}
