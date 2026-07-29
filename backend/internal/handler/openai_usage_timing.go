package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// applyOpenAIForwardTiming snapshots scheduler phases before the result is handed
// to the asynchronous usage recorder. The pre-forward value is captured before
// upstream generation, so E2E TTFT does not include response streaming time.
func applyOpenAIForwardTiming(c *gin.Context, preForwardE2EFirstTokenMs int, result *service.OpenAIForwardResult) {
	if c == nil || result == nil {
		return
	}
	timing := service.OpenAIRequestTimingFromContext(c)
	if timing == nil {
		return
	}
	snapshot := timing.Snapshot()
	result.BodyReadMs = openAIIntPtr(snapshot.BodyReadMS)
	result.PreprocessMs = openAIIntPtr(snapshot.PreprocessMS)
	result.UserQueueMs = openAIIntPtr(snapshot.UserQueueMS)
	result.RoutingMs = openAIIntPtr(snapshot.RoutingMS)
	result.QueueMs = openAIIntPtr(snapshot.QueueMS)
	result.RetryMs = openAIIntPtr(snapshot.RetryMS)
	if result.FirstTokenMs != nil {
		result.E2EFirstTokenMs = openAIIntPtr(preForwardE2EFirstTokenMs + *result.FirstTokenMs)
	}
}

func openAIIntPtr(value int) *int {
	return &value
}
