package admin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type anthropicAutoInspectAdminService interface {
	GetSettings(ctx context.Context) (*service.AnthropicAutoInspectSettings, error)
	UpdateSettings(ctx context.Context, input service.AnthropicAutoInspectSettings) error
	RunNow(ctx context.Context) error
	ListLogs(ctx context.Context, params pagination.PaginationParams, filter service.AnthropicAutoInspectLogFilter) ([]service.AnthropicAutoInspectLog, *pagination.PaginationResult, error)
	ListBatches(ctx context.Context, params pagination.PaginationParams) ([]service.AnthropicAutoInspectBatch, *pagination.PaginationResult, error)
}

type AnthropicAutoInspectHandler struct {
	svc anthropicAutoInspectAdminService
}

func NewAnthropicAutoInspectHandler(svc *service.AnthropicAutoInspectService) *AnthropicAutoInspectHandler {
	return &AnthropicAutoInspectHandler{svc: svc}
}

func (h *AnthropicAutoInspectHandler) GetSettings(c *gin.Context) {
	settings, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AnthropicAutoInspectHandler) UpdateSettings(c *gin.Context) {
	var req service.AnthropicAutoInspectSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 1
	}
	if req.ErrorCooldownMinutes <= 0 {
		req.ErrorCooldownMinutes = 30
	}
	if err := h.svc.UpdateSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}

func (h *AnthropicAutoInspectHandler) RunNow(c *gin.Context) {
	if err := h.svc.RunNow(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "started"})
}

func (h *AnthropicAutoInspectHandler) ListLogs(c *gin.Context) {
	page, pageSize := parseAnthropicAutoInspectPagination(c)
	items, pager, err := h.svc.ListLogs(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.AnthropicAutoInspectLogFilter{
		Search:      strings.TrimSpace(c.Query("search")),
		Result:      service.AnthropicAutoInspectResult(strings.TrimSpace(c.Query("result"))),
		StartedFrom: parseAnthropicAutoInspectTime(c.Query("started_from")),
		StartedTo:   parseAnthropicAutoInspectTime(c.Query("started_to")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"items":      items,
		"pagination": pager,
	})
}

func (h *AnthropicAutoInspectHandler) ListBatches(c *gin.Context) {
	page, pageSize := parseAnthropicAutoInspectPagination(c)
	items, pager, err := h.svc.ListBatches(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"items":      items,
		"pagination": pager,
	})
}

func parseAnthropicAutoInspectPagination(c *gin.Context) (int, int) {
	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize := 20
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	return page, pageSize
}

func parseAnthropicAutoInspectTime(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
