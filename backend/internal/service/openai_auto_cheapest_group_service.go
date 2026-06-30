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
	provider AvailableOpenAIGroupsProvider
}

func NewOpenAIAutoCheapestGroupResolver(provider AvailableOpenAIGroupsProvider) *OpenAIAutoCheapestGroupResolver {
	return &OpenAIAutoCheapestGroupResolver{provider: provider}
}

func EffectiveOpenAIGroupRate(group Group, userRates map[int64]float64) float64 {
	if userRates != nil {
		if rate, ok := userRates[group.ID]; ok {
			return rate
		}
	}
	return group.RateMultiplier
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

func (r *OpenAIAutoCheapestGroupResolver) CandidateGroups(ctx context.Context, userID int64) ([]Group, error) {
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
		if group.Status != StatusActive {
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
