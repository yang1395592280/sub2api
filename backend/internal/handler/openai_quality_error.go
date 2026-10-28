package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OpenAIGatewayHandler) handleOpenAIQualityError(c *gin.Context, err error, started bool) bool {
	if !service.IsOpenAIQualityError(err) {
		return false
	}
	status, code, message := service.OpenAIQualityErrorDetails(err)
	h.handleStreamingAwareErrorWithCode(c, status, "api_error", code, message, started, false)
	return true
}
