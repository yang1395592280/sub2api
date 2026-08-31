package service

import (
	"strings"
	"time"
)

func applyChannelPriceSnapshot(log *UsageLog, account *Account) {
	if log == nil || account == nil || account.ChannelPrice == nil {
		return
	}
	price := *account.ChannelPrice
	log.ChannelPriceSnapshot = &price

	source := "manual"
	if status := account.getExtraString("upstream_balance_status"); status != "" {
		source = "upstream_balance"
	}
	log.ChannelPriceSource = &source

	if updatedAt := account.getExtraString("upstream_balance_updated_at"); updatedAt != "" {
		if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			log.ChannelPriceRefreshedAt = &ts
		}
	}
}

func optionalTrimmedStringPtr(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func usageLogGroupNameSnapshot(apiKey *APIKey) *string {
	if apiKey == nil || apiKey.Group == nil {
		return nil
	}
	return optionalTrimmedStringPtr(apiKey.Group.Name)
}

// optionalNonEqualStringPtr returns a pointer to value if it is non-empty and
// differs from compare; otherwise nil. Used to store upstream_model only when
// it differs from the requested model.
func optionalNonEqualStringPtr(value, compare string) *string {
	if value == "" || value == compare {
		return nil
	}
	return &value
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// coalesceRequestedReasoningEffort prefers the client-requested value and falls
// back to the effective/forwarded effort for historical or unmapped rows.
func coalesceRequestedReasoningEffort(requested, forwarded *string) *string {
	if trimmed := optionalStringValue(requested); trimmed != "" {
		return &trimmed
	}
	if trimmed := optionalStringValue(forwarded); trimmed != "" {
		return &trimmed
	}
	return nil
}

func forwardResultBillingModel(requestedModel, upstreamModel string) string {
	if trimmed := strings.TrimSpace(requestedModel); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(upstreamModel)
}

func optionalInt64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
