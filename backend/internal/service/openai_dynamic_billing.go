package service

import (
	"context"
	"math"
)

// OpenAIDynamicBillingRange returns the user-visible multiplier interval. The
// same upstream grouping range remains the single source of truth for account
// movement; fixed profit only shifts the displayed and billed range.
func OpenAIDynamicBillingRange(group *Group, profitMarkup float64) (float64, float64, bool) {
	if group == nil || !group.DynamicBillingEnabled || !group.UpstreamPriceGroupingEnabled || group.Platform != PlatformOpenAI || group.GroupRole != GroupRoleStandard || group.SubscriptionType != SubscriptionTypeStandard {
		return 0, 0, false
	}
	if profitMarkup < 0 || math.IsNaN(profitMarkup) || math.IsInf(profitMarkup, 0) {
		profitMarkup = 0
	}
	minRate := group.UpstreamPriceGroupingMin + profitMarkup
	maxRate := group.UpstreamPriceGroupingMax + profitMarkup
	if maxRate <= 0 || maxRate < minRate {
		return 0, 0, false
	}
	return minRate, maxRate, true
}

// ResolveOpenAIDynamicBillingMultiplier replaces the static group multiplier
// only for an enabled OpenAI dynamic group. Accounts without channel pricing
// (notably self-hosted pool accounts) use the selected standard group's maximum.
func ResolveOpenAIDynamicBillingMultiplier(ctx context.Context, apiKey *APIKey, account *Account, settings *SettingService, fallback float64) float64 {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.DynamicBillingEnabled {
		return fallback
	}
	profitMarkup := 0.0
	if settings != nil {
		profitMarkup = settings.GetOpenAIDynamicBillingProfitMarkup(ctx)
	}
	_, maxRate, enabled := OpenAIDynamicBillingRange(apiKey.Group, profitMarkup)
	if !enabled {
		return fallback
	}
	if account == nil || account.ChannelPrice == nil {
		return maxRate
	}
	rate := *account.ChannelPrice + profitMarkup
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return maxRate
	}
	return math.Min(rate, maxRate)
}
