package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

type OpenAIAutoSchedulerSelector struct {
	service openAIAutoSchedulerSelectorService
}

type openAIAutoSchedulerSelectorService interface {
	IsEnabledForGroup(ctx context.Context, groupID *int64) bool
	GetStateForSelection(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error)
	HasOpenCircuitForSelection(ctx context.Context, accountID, groupID int64, model string) (bool, error)
	GetSettingsForSelection(ctx context.Context) OpenAIAutoSchedulerSettings
}

func NewOpenAIAutoSchedulerSelector(service openAIAutoSchedulerSelectorService) *OpenAIAutoSchedulerSelector {
	return &OpenAIAutoSchedulerSelector{service: service}
}

func (s *OpenAIAutoSchedulerSelector) Rank(ctx context.Context, groupID *int64, requestedModel string, candidates []*Account) ([]*Account, bool) {
	if s == nil || s.service == nil || groupID == nil || !s.service.IsEnabledForGroup(ctx, groupID) {
		return candidates, false
	}
	if len(candidates) == 0 {
		return candidates, true
	}

	now := time.Now()
	settings := s.service.GetSettingsForSelection(ctx)
	probeModel := normalizeOpenAIAutoSchedulerSettings(settings).ProbeModel
	minPrice, maxPrice := openAIAutoSchedulerCandidatePriceRange(candidates)
	ranked := make([]openAIAutoSchedulerRankedAccount, 0, len(candidates))
	for index, account := range candidates {
		if account == nil {
			continue
		}
		state, err := s.service.GetStateForSelection(ctx, account.ID, *groupID, probeModel)
		if err != nil {
			return candidates, false
		}
		if state == nil {
			neutral := NewOpenAIAutoSchedulerScoreState(account.ID, *groupID, probeModel)
			state = &neutral
		}
		if isOpenAIAutoSchedulerStateInActiveCooldown(*state, now) {
			continue
		}
		openCircuit, err := s.service.HasOpenCircuitForSelection(ctx, account.ID, *groupID, probeModel)
		if err != nil {
			return candidates, false
		}
		if openCircuit {
			continue
		}
		ranked = append(ranked, openAIAutoSchedulerRankedAccount{
			account:        account,
			state:          *state,
			effectiveScore: openAIAutoSchedulerEffectiveSelectionScore(*state, account, minPrice, maxPrice, settings),
			index:          index,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if tierA, tierB := openAIAutoSchedulerStateTier(a.state), openAIAutoSchedulerStateTier(b.state); tierA != tierB {
			return tierA < tierB
		}
		if openAIAutoSchedulerStateTier(a.state) == openAIAutoSchedulerStateTier(OpenAIAutoSchedulerScoreState{State: OpenAIAutoSchedulerStateRunning}) {
			speedA, okA := openAIAutoSchedulerSpeedMS(a.state)
			speedB, okB := openAIAutoSchedulerSpeedMS(b.state)
			if okA != okB {
				return okA
			}
			if okA && speedA != speedB {
				return speedA < speedB
			}
		}
		if a.effectiveScore != b.effectiveScore {
			return a.effectiveScore > b.effectiveScore
		}
		if a.account.Priority != b.account.Priority {
			return a.account.Priority < b.account.Priority
		}
		switch {
		case a.account.LastUsedAt == nil && b.account.LastUsedAt != nil:
			return true
		case a.account.LastUsedAt != nil && b.account.LastUsedAt == nil:
			return false
		case a.account.LastUsedAt != nil && b.account.LastUsedAt != nil && !a.account.LastUsedAt.Equal(*b.account.LastUsedAt):
			return a.account.LastUsedAt.Before(*b.account.LastUsedAt)
		default:
			return a.index < b.index
		}
	})

	out := make([]*Account, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.account)
	}
	return out, true
}

func (s *OpenAIAutoSchedulerSelector) IsAccountTemporarilyBlocked(ctx context.Context, groupID *int64, requestedModel string, accountID int64) bool {
	if s == nil || s.service == nil || groupID == nil || accountID <= 0 || !s.service.IsEnabledForGroup(ctx, groupID) {
		return false
	}
	settings := s.service.GetSettingsForSelection(ctx)
	probeModel := normalizeOpenAIAutoSchedulerSettings(settings).ProbeModel
	state, err := s.service.GetStateForSelection(ctx, accountID, *groupID, probeModel)
	if err != nil {
		return false
	}
	if state == nil {
		openCircuit, activeErr := s.service.HasOpenCircuitForSelection(ctx, accountID, *groupID, probeModel)
		return activeErr == nil && openCircuit
	}
	if isOpenAIAutoSchedulerStateInActiveCooldown(*state, time.Now()) {
		return true
	}
	openCircuit, activeErr := s.service.HasOpenCircuitForSelection(ctx, accountID, *groupID, probeModel)
	return activeErr == nil && openCircuit
}

func isOpenAIAutoSchedulerStateInActiveCooldown(state OpenAIAutoSchedulerScoreState, now time.Time) bool {
	return state.State == OpenAIAutoSchedulerStateOpen &&
		(state.CooldownUntil == nil || state.CooldownUntil.After(now))
}

type openAIAutoSchedulerRankedAccount struct {
	account        *Account
	state          OpenAIAutoSchedulerScoreState
	effectiveScore int
	index          int
}

func openAIAutoSchedulerStateTier(state OpenAIAutoSchedulerScoreState) int {
	switch state.State {
	case OpenAIAutoSchedulerStateHalfOpen:
		return 0
	case OpenAIAutoSchedulerStateRunning, "":
		return 1
	case OpenAIAutoSchedulerStateObserving:
		return 2
	case OpenAIAutoSchedulerStateOpen:
		return 3
	default:
		return 1
	}
}

func (s *OpenAIAutoSchedulerService) GetStateForSelection(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error) {
	if s == nil || s.repo == nil || accountID <= 0 || groupID <= 0 {
		return nil, nil
	}
	return s.repo.GetScoreState(ctx, accountID, groupID, strings.TrimSpace(model))
}

func (s *OpenAIAutoSchedulerService) HasOpenCircuitForSelection(ctx context.Context, accountID, groupID int64, model string) (bool, error) {
	if s == nil || s.repo == nil || accountID <= 0 || groupID <= 0 {
		return false, nil
	}
	return s.repo.HasOpenCircuitScoreState(ctx, accountID, groupID, strings.TrimSpace(model))
}

func (s *OpenAIAutoSchedulerService) GetSettingsForSelection(ctx context.Context) OpenAIAutoSchedulerSettings {
	return s.settings(ctx)
}

func openAIAutoSchedulerCandidatePriceRange(candidates []*Account) (float64, float64) {
	minPrice := math.Inf(1)
	maxPrice := 0.0
	for _, account := range candidates {
		if account == nil {
			continue
		}
		price := account.EffectiveChannelPrice()
		if price < minPrice {
			minPrice = price
		}
		if price > maxPrice {
			maxPrice = price
		}
	}
	if math.IsInf(minPrice, 1) {
		return 1, 1
	}
	return minPrice, maxPrice
}

func openAIAutoSchedulerEffectiveSelectionScore(state OpenAIAutoSchedulerScoreState, account *Account, minPrice, maxPrice float64, settings OpenAIAutoSchedulerSettings) int {
	if maxPrice <= minPrice {
		return state.FinalScore
	}
	price := account.EffectiveChannelPrice()
	cheapness := 1 - ((price - minPrice) / (maxPrice - minPrice))
	costScore := int(math.Round((cheapness*2 - 1) * 1000))
	return clampScore(state.FinalScore + int(float64(costScore)*normalizeOpenAIAutoSchedulerSettings(settings).CostWeight))
}
