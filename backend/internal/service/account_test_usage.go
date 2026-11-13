package service

import "github.com/gin-gonic/gin"

func recordAccountTestResponseMetadata(c *gin.Context, data map[string]any) {
	usage := accountTestTokenUsage(c)
	if model, ok := data["model"].(string); ok && model != "" {
		usage.ModelID = model
	}
	if tier, ok := data["service_tier"].(string); ok {
		usage.ServiceTier = tier
	}
	c.Set(accountTestTokenUsageKey, usage)
}

func recordAccountTestCacheUsage(c *gin.Context, data map[string]any, protocol string) {
	usage := accountTestTokenUsage(c)
	set := func(target **int64, value *int64) {
		if value != nil {
			*target = value
		}
	}
	switch protocol {
	case "anthropic":
		set(&usage.CacheReadTokens, accountTestUsageCount(data, "cache_read_input_tokens"))
		set(&usage.CacheCreationTokens, accountTestUsageCount(data, "cache_creation_input_tokens"))
		if creation, ok := data["cache_creation"].(map[string]any); ok {
			set(&usage.CacheCreation5mTokens, accountTestUsageCount(creation, "ephemeral_5m_input_tokens"))
			set(&usage.CacheCreation1hTokens, accountTestUsageCount(creation, "ephemeral_1h_input_tokens"))
		}
	case "gemini":
		set(&usage.CacheReadTokens, accountTestUsageCount(data, "cachedContentTokenCount"))
		set(&usage.ThinkingTokens, accountTestUsageCount(data, "thoughtsTokenCount"))
	default:
		key := "input_tokens_details"
		if protocol == "chat" {
			key = "prompt_tokens_details"
		}
		if details, ok := data[key].(map[string]any); ok {
			set(&usage.CacheReadTokens, accountTestUsageCount(details, "cached_tokens"))
			set(&usage.CacheCreationTokens, accountTestUsageCount(details, "cache_creation_tokens"))
			set(&usage.CacheCreation5mTokens, accountTestUsageCount(details, "cache_creation_5m_tokens"))
			set(&usage.CacheCreation12hTokens, accountTestUsageCount(details, "cache_creation_12h_tokens"))
		}
	}
	c.Set(accountTestTokenUsageKey, usage)
}
