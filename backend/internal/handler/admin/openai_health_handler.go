package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OpenAIHealthHandler 提供 OpenAI 健康看板聚合接口。
type OpenAIHealthHandler struct {
	monitorService *service.ChannelMonitorService
}

// NewOpenAIHealthHandler 创建 OpenAI 健康看板 handler。
func NewOpenAIHealthHandler(monitorService *service.ChannelMonitorService) *OpenAIHealthHandler {
	return &OpenAIHealthHandler{monitorService: monitorService}
}

// GetOverview GET /api/v1/admin/openai-health/overview
func (h *OpenAIHealthHandler) GetOverview(c *gin.Context) {
	overview, err := h.monitorService.GetOpenAIHealthOverview(c.Request.Context(), service.OpenAIHealthQuery{
		GroupName: strings.TrimSpace(c.Query("group_name")),
		Search:    strings.TrimSpace(c.Query("search")),
		Window:    strings.TrimSpace(c.Query("window")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}
