package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeAutoSchedulerSelectorService struct {
	enabledGroups    map[int64]bool
	settings         OpenAIAutoSchedulerSettings
	states           map[int64]OpenAIAutoSchedulerScoreState
	statesByKey      map[string]OpenAIAutoSchedulerScoreState
	openCircuit      map[int64]bool
	openCircuitByKey map[string]bool
	stateLookups     []string
	circuitLookups   []string
	err              error
}

func (s *fakeAutoSchedulerSelectorService) IsEnabledForGroup(_ context.Context, groupID *int64) bool {
	if s == nil || groupID == nil {
		return false
	}
	return s.enabledGroups[*groupID]
}

func (s *fakeAutoSchedulerSelectorService) GetStateForSelection(_ context.Context, accountID, _ int64, model string) (*OpenAIAutoSchedulerScoreState, error) {
	if s != nil && s.err != nil {
		return nil, s.err
	}
	if s != nil {
		s.stateLookups = append(s.stateLookups, selectorStateKey(accountID, model))
	}
	if s.statesByKey != nil {
		if state, ok := s.statesByKey[selectorStateKey(accountID, model)]; ok {
			return &state, nil
		}
	}
	state, ok := s.states[accountID]
	if !ok {
		return nil, nil
	}
	return &state, nil
}

func (s *fakeAutoSchedulerSelectorService) HasOpenCircuitForSelection(_ context.Context, accountID, _ int64, model string) (bool, error) {
	if s != nil && s.err != nil {
		return false, s.err
	}
	if s != nil {
		key := selectorStateKey(accountID, model)
		s.circuitLookups = append(s.circuitLookups, key)
		if s.openCircuitByKey != nil {
			return s.openCircuitByKey[key], nil
		}
	}
	return s != nil && s.openCircuit[accountID], nil
}

func (s *fakeAutoSchedulerSelectorService) GetSettingsForSelection(context.Context) OpenAIAutoSchedulerSettings {
	if s == nil {
		return enabledOpenAIAutoSchedulerSettings()
	}
	if s.settings.ProbeIntervalSeconds == 0 {
		settings := enabledOpenAIAutoSchedulerSettings()
		settings.CostWeight = 1
		return settings
	}
	return s.settings
}

func TestOpenAIAutoSchedulerSelector_GroupGate(t *testing.T) {
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{enabledGroups: map[int64]bool{10: true}})
	accounts := []*Account{{ID: 1}, {ID: 2}}

	ranked, used := selector.Rank(context.Background(), nil, "gpt-5", accounts)
	require.False(t, used)
	require.Equal(t, accounts, ranked)

	groupID := int64(10)
	ranked, used = selector.Rank(context.Background(), &groupID, "gpt-5", accounts)
	require.True(t, used)
	require.Len(t, ranked, 2)
}

func TestOpenAIAutoSchedulerSelector_SkipsOpenCircuitAndSortsByScore(t *testing.T) {
	groupID := int64(10)
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{
		enabledGroups: map[int64]bool{10: true},
		states: map[int64]OpenAIAutoSchedulerScoreState{
			1: {AccountID: 1, GroupID: 10, Model: "gpt-5", FinalScore: 1000, State: OpenAIAutoSchedulerStateOpen, CooldownUntil: ptrSelectorTime(time.Now().Add(time.Minute))},
			2: {AccountID: 2, GroupID: 10, Model: "gpt-5", FinalScore: 9000, State: OpenAIAutoSchedulerStateRunning},
			3: {AccountID: 3, GroupID: 10, Model: "gpt-5", FinalScore: 7000, State: OpenAIAutoSchedulerStateRunning},
		},
	})
	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-5", []*Account{{ID: 1}, {ID: 3}, {ID: 2}})
	require.True(t, used)
	require.Equal(t, []int64{2, 3}, selectorAccountIDs(ranked))
}

func TestOpenAIAutoSchedulerSelector_UsesProbeModelForStateAndCircuit(t *testing.T) {
	groupID := int64(10)
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	svc := &fakeAutoSchedulerSelectorService{
		enabledGroups: map[int64]bool{10: true},
		settings:      settings,
		statesByKey: map[string]OpenAIAutoSchedulerScoreState{
			selectorStateKey(1, "gpt-5.4"): {AccountID: 1, GroupID: 10, Model: "gpt-5.4", FinalScore: 9000, State: OpenAIAutoSchedulerStateOpen},
			selectorStateKey(1, "gpt-5.5"): {AccountID: 1, GroupID: 10, Model: "gpt-5.5", FinalScore: 9000, State: OpenAIAutoSchedulerStateRunning},
			selectorStateKey(2, "gpt-5.5"): {AccountID: 2, GroupID: 10, Model: "gpt-5.5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
		},
		openCircuitByKey: map[string]bool{
			selectorStateKey(1, "gpt-5.4"): true,
		},
	}
	selector := NewOpenAIAutoSchedulerSelector(svc)

	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-4o", []*Account{{ID: 1}, {ID: 2}})

	require.True(t, used)
	require.Equal(t, []int64{1, 2}, selectorAccountIDs(ranked))
	require.Contains(t, svc.stateLookups, selectorStateKey(1, "gpt-5.5"))
	require.Contains(t, svc.circuitLookups, selectorStateKey(1, "gpt-5.5"))
}

func TestOpenAIAutoSchedulerSelector_MissingStateUsesNeutralScoreAndStableFallbacks(t *testing.T) {
	groupID := int64(10)
	oldUsed := time.Now().Add(-2 * time.Hour)
	newUsed := time.Now().Add(-1 * time.Hour)
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{
		enabledGroups: map[int64]bool{10: true},
		states: map[int64]OpenAIAutoSchedulerScoreState{
			1: {AccountID: 1, GroupID: 10, Model: "gpt-5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
			2: {AccountID: 2, GroupID: 10, Model: "gpt-5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
		},
	})

	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-5", []*Account{
		{ID: 3, Priority: 1, LastUsedAt: &newUsed},
		{ID: 1, Priority: 1, LastUsedAt: &oldUsed},
		{ID: 2, Priority: 2},
	})

	require.True(t, used)
	require.Equal(t, []int64{1, 3, 2}, selectorAccountIDs(ranked))
}

func TestOpenAIAutoSchedulerSelector_ServiceErrorPreservesOriginalOrder(t *testing.T) {
	groupID := int64(10)
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{
		enabledGroups: map[int64]bool{10: true},
		err:           errors.New("score cache unavailable"),
	})
	accounts := []*Account{{ID: 2}, {ID: 1}}

	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-5", accounts)

	require.False(t, used)
	require.Equal(t, accounts, ranked)
}

func TestOpenAIAutoSchedulerSelector_ChannelPriceChangesRankingWithinCandidateSet(t *testing.T) {
	groupID := int64(10)
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.CostWeight = 1
	cheapPrice := 0.25
	expensivePrice := 2.0
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{
		enabledGroups: map[int64]bool{10: true},
		settings:      settings,
		states: map[int64]OpenAIAutoSchedulerScoreState{
			1: {AccountID: 1, GroupID: 10, Model: "gpt-5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
			2: {AccountID: 2, GroupID: 10, Model: "gpt-5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
		},
	})

	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-5", []*Account{
		{ID: 1, ChannelPrice: &expensivePrice},
		{ID: 2, ChannelPrice: &cheapPrice},
	})

	require.True(t, used)
	require.Equal(t, []int64{2, 1}, selectorAccountIDs(ranked))
}

func selectorAccountIDs(accounts []*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func ptrSelectorTime(value time.Time) *time.Time {
	return &value
}

func selectorStateKey(accountID int64, model string) string {
	return fmt.Sprintf("%d:%s", accountID, model)
}
