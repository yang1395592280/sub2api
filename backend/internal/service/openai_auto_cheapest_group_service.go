package service

import (
	"context"
	"sort"
	"time"
)

type AvailableOpenAIGroupsProvider interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
	GetUserGroupRates(ctx context.Context, userID int64) (map[int64]float64, error)
}

type LastEffectiveGroupUpdater interface {
	UpdateLastEffectiveGroup(ctx context.Context, apiKeyID int64, groupID int64, at time.Time) error
}

type OpenAIAutoCheapestGroupResolver struct {
	provider       AvailableOpenAIGroupsProvider
	settingService *SettingService
}

func NewOpenAIAutoCheapestGroupResolver(provider AvailableOpenAIGroupsProvider, settings ...*SettingService) *OpenAIAutoCheapestGroupResolver {
	resolver := &OpenAIAutoCheapestGroupResolver{provider: provider}
	if len(settings) > 0 {
		resolver.settingService = settings[0]
	}
	return resolver
}

func EffectiveOpenAIGroupRate(group Group, userRates map[int64]float64) float64 {
	if minRate, _, enabled := OpenAIDynamicBillingRange(&group, 0); enabled {
		// Fixed profit is global, so sorting dynamic tiers by their raw lower bound
		// produces the same order as sorting by their displayed minimum.
		return minRate
	}
	if userRates != nil {
		if rate, ok := userRates[group.ID]; ok {
			return rate
		}
	}
	return group.RateMultiplier
}

func (r *OpenAIAutoCheapestGroupResolver) maximumAcceptedOpenAIGroupRate(ctx context.Context, group Group, userRates map[int64]float64) float64 {
	profitMarkup := 0.0
	if r != nil && r.settingService != nil {
		profitMarkup = r.settingService.GetOpenAIDynamicBillingProfitMarkup(ctx)
	}
	if _, maxRate, enabled := OpenAIDynamicBillingRange(&group, profitMarkup); enabled {
		return maxRate
	}
	return EffectiveOpenAIGroupRate(group, userRates)
}

func CloneAPIKeyForEffectiveGroup(apiKey *APIKey, group *Group) *APIKey {
	if apiKey == nil || group == nil {
		return apiKey
	}

	cp := *apiKey
	gid := group.ID
	groupCopy := *group
	cp.GroupID = &gid
	cp.Group = &groupCopy

	return &cp
}

func SelectFirstEffectiveOpenAIGroupForTest(apiKey *APIKey, groups []Group, try func(*APIKey) error) (*APIKey, error) {
	if apiKey == nil {
		return nil, ErrNoAvailableAccounts
	}
	var lastErr error
	for i := range groups {
		effectiveKey := CloneAPIKeyForEffectiveGroup(apiKey, &groups[i])
		if try == nil {
			return effectiveKey, nil
		}
		if err := try(effectiveKey); err != nil {
			lastErr = err
			continue
		}
		return effectiveKey, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoAvailableAccounts
}

func (r *OpenAIAutoCheapestGroupResolver) CandidateGroups(ctx context.Context, userID int64, maxRateMultiplier *float64) ([]Group, error) {
	if r == nil || r.provider == nil || userID <= 0 {
		return nil, nil
	}

	groups, err := r.provider.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	userRates, err := r.provider.GetUserGroupRates(ctx, userID)
	if err != nil {
		return nil, err
	}

	candidates := make([]Group, 0, len(groups))
	for _, group := range groups {
		if group.Platform != PlatformOpenAI {
			continue
		}
		if group.IsSelfHostedPool() {
			continue
		}
		if group.Status != StatusActive {
			continue
		}
		if !group.AllowAutoCheapestScheduling {
			continue
		}
		if maxRateMultiplier != nil && *maxRateMultiplier > 0 && r.maximumAcceptedOpenAIGroupRate(ctx, group, userRates) > *maxRateMultiplier {
			continue
		}
		candidates = append(candidates, group)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftRate := EffectiveOpenAIGroupRate(candidates[i], userRates)
		rightRate := EffectiveOpenAIGroupRate(candidates[j], userRates)
		if leftRate != rightRate {
			return leftRate < rightRate
		}
		if candidates[i].SortOrder != candidates[j].SortOrder {
			return candidates[i].SortOrder < candidates[j].SortOrder
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates, nil
}
