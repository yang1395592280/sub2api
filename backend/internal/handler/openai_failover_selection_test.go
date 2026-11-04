//go:build unit

package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func fixedGroupRetryKey() *service.APIKey {
	id := int64(20)
	return &service.APIKey{GroupID: &id, Group: &service.Group{ID: id, Platform: service.PlatformOpenAI}}
}

func TestOpenAIFailoverSelection_ReusesAlternativeAndTransfersSlot(t *testing.T) {
	var state openAIFailoverSelection
	defer state.Close()
	key := fixedGroupRetryKey()
	failed := map[int64]struct{}{1: {}}
	account := &service.Account{ID: 20, Platform: service.PlatformOpenAI}
	released, selections := 0, 0
	result := &service.AccountSelectionResult{Account: &service.Account{ID: 30}, Acquired: true, ReleaseFunc: func() { released++ }}
	selectAccount := func(excluded map[int64]struct{}) (*service.APIKey, string, *service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
		selections++
		require.Contains(t, excluded, int64(1))
		require.Contains(t, excluded, account.ID)
		return key, "mapped-model", result, service.OpenAIAccountScheduleDecision{SelectedAccountID: 30}, nil
	}
	require.True(t, state.TryAlternative(context.Background(), key, account, &service.UpstreamFailoverError{}, failed, false, true, selectAccount))
	require.Equal(t, map[int64]struct{}{1: {}}, failed, "probing must not mutate the request's failure set")
	selectedKey, model, selection, decision, err := state.Select(failed, selectAccount)
	require.NoError(t, err)
	require.Same(t, key, selectedKey)
	require.Equal(t, "mapped-model", model)
	require.Same(t, result, selection)
	require.Equal(t, int64(30), decision.SelectedAccountID)
	require.Equal(t, 1, selections, "consume the acquired selection without another scheduler call")
	state.Close()
	require.Zero(t, released, "the handler owns a consumed slot")
	selection.ReleaseFunc()
	require.Equal(t, 1, released)
}

func TestOpenAIFailoverSelection_ReleasesUnusedOrRejectedAlternative(t *testing.T) {
	for _, outcome := range []string{"unused", "selection_error", "already_failed", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			var state openAIFailoverSelection
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			released := 0
			selector := func(map[int64]struct{}) (*service.APIKey, string, *service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
				id := int64(30)
				if outcome == "already_failed" {
					id = 20
				}
				selection := &service.AccountSelectionResult{Account: &service.Account{ID: id}, Acquired: true, ReleaseFunc: func() { released++ }}
				var err error
				if outcome == "selection_error" {
					err = errors.New("selection unavailable")
				}
				if outcome == "canceled" {
					cancel()
				}
				return fixedGroupRetryKey(), "", selection, service.OpenAIAccountScheduleDecision{}, err
			}
			found := state.TryAlternative(ctx, fixedGroupRetryKey(), &service.Account{ID: 20, Platform: service.PlatformOpenAI}, &service.UpstreamFailoverError{}, nil, false, true, selector)
			require.Equal(t, outcome == "unused", found)
			state.Close()
			state.Close()
			require.Equal(t, 1, released, "release each unconsumed concurrency slot exactly once")
		})
	}
}

func TestOpenAIFailoverSelection_PreservesBindingCapacityAndSwitchBudget(t *testing.T) {
	for _, guard := range []string{"binding", "capacity", "switch_budget", "automatic", "other_platform", "stopped", "canceled", "no_alternative"} {
		t.Run(guard, func(t *testing.T) {
			var state openAIFailoverSelection
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			key := fixedGroupRetryKey()
			failure := &service.UpstreamFailoverError{}
			if guard == "automatic" {
				key.GroupSelectMode = service.APIKeyGroupSelectModeOpenAIAutoCheapest
			}
			if guard == "capacity" {
				failure.RequestScopedTransient = true
			}
			if guard == "other_platform" {
				key.Group.Platform = service.PlatformGrok
			}
			if guard == "stopped" {
				failure.NextAccountAction = service.NextAccountStop
			}
			if guard == "canceled" {
				cancel()
			}
			calls := 0
			selector := func(map[int64]struct{}) (*service.APIKey, string, *service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
				calls++
				return nil, "", nil, service.OpenAIAccountScheduleDecision{}, service.ErrNoAvailableAccounts
			}
			require.False(t, state.TryAlternative(ctx, key, &service.Account{ID: 20, Platform: service.PlatformOpenAI}, failure, nil, guard == "binding", guard != "switch_budget", selector))
			if guard == "no_alternative" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
