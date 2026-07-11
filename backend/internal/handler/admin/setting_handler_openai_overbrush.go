package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetOpenAIOverbrushSettings 获取 OpenAI OAuth 超刷配置。
// GET /api/v1/admin/settings/openai-overbrush
func (h *SettingHandler) GetOpenAIOverbrushSettings(c *gin.Context) {
	settings, err := h.settingService.GetOpenAIOverbrushSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OpenAIOverbrushSettings{
		Consecutive429Threshold: settings.Consecutive429Threshold,
	})
}

type UpdateOpenAIOverbrushSettingsRequest struct {
	Consecutive429Threshold int `json:"consecutive_429_threshold"`
}

// UpdateOpenAIOverbrushSettings 更新 OpenAI OAuth 超刷配置。
// PUT /api/v1/admin/settings/openai-overbrush
func (h *SettingHandler) UpdateOpenAIOverbrushSettings(c *gin.Context) {
	var req UpdateOpenAIOverbrushSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.OpenAIOverbrushSettings{
		Consecutive429Threshold: req.Consecutive429Threshold,
	}
	if err := h.settingService.SetOpenAIOverbrushSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedSettings, err := h.settingService.GetOpenAIOverbrushSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OpenAIOverbrushSettings{
		Consecutive429Threshold: updatedSettings.Consecutive429Threshold,
	})
}
