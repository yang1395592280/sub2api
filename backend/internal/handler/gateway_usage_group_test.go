package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyAPIKeyGroupUsageFields(t *testing.T) {
	groupID := int64(2)
	resp := gin.H{}

	applyAPIKeyGroupUsageFields(resp, &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:             2,
			Name:           "GPT Plus",
			Platform:       service.PlatformOpenAI,
			RateMultiplier: 0.08,
		},
	})

	require.Equal(t, int64(2), resp["group_id"])
	require.Equal(t, gin.H{
		"id":              int64(2),
		"name":            "GPT Plus",
		"platform":        service.PlatformOpenAI,
		"rate_multiplier": 0.08,
	}, resp["group"])
}
