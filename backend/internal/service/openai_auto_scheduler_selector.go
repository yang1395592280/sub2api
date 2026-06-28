package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

const openAIAutoSchedulerNeutralSelectionScore = 6000

type OpenAIAutoSchedulerSelector struct {
	service openAIAutoSchedulerSelectorService
}

type openAIAutoSchedulerSelectorService interface {
	IsEnabledForGroup(ctx context.Context, groupID *int64) bool
	GetStateForSelection(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error)
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
	ranked := make([]openAIAutoSchedulerRankedAccount, 0, len(candidates))
	for index, account := range candidates {
		if account == nil {
			continue
		}
		state, err := s.service.GetStateForSelection(ctx, account.ID, *groupID, strings.TrimSpace(requestedModel))
		if err != nil {
			return candidates, false
		}
		if state == nil {
			neutral := NewOpenAIAutoSchedulerScoreState(account.ID, *groupID, requestedModel)
			state = &neutral
		}
		if state.State == OpenAIAutoSchedulerStateOpen &&
			(state.CooldownUntil == nil || state.CooldownUntil.After(now)) {
			continue
		}
		ranked = append(ranked, openAIAutoSchedulerRankedAccount{
			account: account,
			state:   *state,
			index:   index,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if tierA, tierB := openAIAutoSchedulerStateTier(a.state), openAIAutoSchedulerStateTier(b.state); tierA != tierB {
			return tierA < tierB
		}
		if a.state.FinalScore != b.state.FinalScore {
			return a.state.FinalScore > b.state.FinalScore
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

type openAIAutoSchedulerRankedAccount struct {
	account *Account
	state   OpenAIAutoSchedulerScoreState
	index   int
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
