package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetCustomFeatures(c *gin.Context) {
	result, err := h.settingService.GetCustomFeatures(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *SettingHandler) PatchCustomFeatures(c *gin.Context) {
	var req struct {
		Flags map[string]bool `json:"flags"`
		Menus map[string]bool `json:"menus"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid feature settings")
		return
	}
	if err := h.settingService.PatchCustomFeatures(c.Request.Context(), req.Flags, req.Menus); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.GetCustomFeatures(c)
}
