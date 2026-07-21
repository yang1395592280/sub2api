package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEffectiveOpenAIGroupRate_UsesUserRateOverride(t *testing.T) {
	group := Group{ID: 10, RateMultiplier: 0.3}
	got := EffectiveOpenAIGroupRate(group, map[int64]float64{10: 0.12})
	require.Equal(t, 0.12, got)
}

func TestOpenAIAutoCheapestSelection_QualityFirstAcrossGroups(t *testing.T) {
	cheapGroupID := int64(111)
	expensiveGroupID := int64(112)
	cheapSlow := Account{ID: 1101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID}}
	cheapFast := Account{ID: 1102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID}}
	expensiveFast := Account{ID: 1201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{expensiveGroupID}}
	circuit := &autoCheapestCircuitStub{}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{cheapSlow, cheapFast, expensiveFast}}},
		cache:       &schedulerTestGatewayCache{}, cfg: newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			cheapSlow.ID: true, cheapFast.ID: true, expensiveFast.ID: true,
		}}),
	}
	svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: []Group{
		{ID: cheapGroupID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1},
		{ID: expensiveGroupID, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2},
	}}), nil)
	svc.SetOpenAIAutoCheapestGroupCircuit(circuit)

	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.6-sol", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportAny,
	}
	now := time.Now()
	lastRealAt := now.Add(-time.Second)
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	for _, sample := range []struct {
		account *Account
		ttft    float64
	}{
		{account: &cheapSlow, ttft: 15_000},
		{account: &cheapFast, ttft: 500},
		{account: &expensiveFast, ttft: 300},
	} {
		key := svc.openAIBalancedHealthKeyForCandidate(sample.account, req)
		healthRepo.states[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: sample.ttft,
			RealSampleCount: 2, LastRealAt: &lastRealAt, ExpiresAt: now.Add(time.Hour),
		}
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))
	apiKey := &APIKey{ID: 1, UserID: 9, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)

	effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, nil, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)

	require.NoError(t, err)
	require.Equal(t, &cheapGroupID, effectiveKey.GroupID)
	require.Equal(t, cheapFast.ID, selection.Account.ID)
	require.Zero(t, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	// Once the only qualified cheap account failed, the slow cheap account must
	// not delay the request before the qualified stable group is attempted.
	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, map[int64]struct{}{cheapFast.ID: {}}, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Equal(t, &expensiveGroupID, effectiveKey.GroupID)
	require.Equal(t, expensiveFast.ID, selection.Account.ID)
	require.Zero(t, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	RequireOpenAIAutoCheapestQualifiedFailover(ctx)
	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, map[int64]struct{}{cheapFast.ID: {}, expensiveFast.ID: {}}, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.Error(t, err)
	require.Nil(t, effectiveKey)
	require.Nil(t, selection)
	require.Zero(t, circuit.calls)
}

func TestOpenAIAutoCheapestSelection_FirstOutputTimeoutDisablesLowQualityFallback(t *testing.T) {
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true)
	require.False(t, openAIAutoCheapestRequiresQualifiedFailover(ctx))

	RequireOpenAIAutoCheapestQualifiedFailover(ctx)

	require.True(t, openAIAutoCheapestRequiresQualifiedFailover(ctx))
}

func TestCloneAPIKeyForEffectiveGroup_UsesActualGroup(t *testing.T) {
	gid := int64(82)
	src := &APIKey{
		ID:              7,
		UserID:          9,
		GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest,
		GroupID:         nil,
		Group:           nil,
	}
	group := &Group{ID: gid, Name: "plus", Platform: PlatformOpenAI, RateMultiplier: 0.1, Hydrated: true}

	got := CloneAPIKeyForEffectiveGroup(src, group)

	require.NotSame(t, src, got)
	require.Equal(t, &gid, got.GroupID)
	require.NotNil(t, got.Group)
	require.Equal(t, group.ID, got.Group.ID)
	require.Equal(t, APIKeyGroupSelectModeOpenAIAutoCheapest, got.GroupSelectMode)
}

func TestOpenAIAutoCheapestSelection_TriesNextGroupWhenCheapestUnavailable(t *testing.T) {
	cheapID := int64(10)
	nextID := int64(20)
	apiKey := &APIKey{ID: 1, UserID: 9, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	groups := []Group{
		{ID: cheapID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1},
		{ID: nextID, Name: "next", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2},
	}
	var attempted []int64
	got, err := SelectFirstEffectiveOpenAIGroupForTest(apiKey, groups, func(effective *APIKey) error {
		require.NotNil(t, effective.GroupID)
		attempted = append(attempted, *effective.GroupID)
		if *effective.GroupID == cheapID {
			return ErrNoAvailableAccounts
		}
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, []int64{cheapID, nextID}, attempted)
	require.Equal(t, &nextID, got.GroupID)
	require.Nil(t, apiKey.GroupID)
}

func TestOpenAIAutoCheapestSelection_ExhaustsCheapGroupBeforeHigherRateGroup(t *testing.T) {
	cheapGroupID := int64(101)
	expensiveGroupID := int64(102)
	cheapFailed := Account{ID: 1001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID}}
	cheapAvailable := Account{ID: 1002, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID}}
	expensiveAvailable := Account{ID: 2001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{expensiveGroupID}}
	circuit := &autoCheapestCircuitStub{}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{
			cheapFailed,
			cheapAvailable,
			expensiveAvailable,
		}}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: cheapGroupID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1},
			{ID: expensiveGroupID, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.15},
		},
	}), nil)
	svc.SetOpenAIAutoCheapestGroupCircuit(circuit)
	apiKey := &APIKey{ID: 1, UserID: 9, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true, circuit)

	effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx,
		apiKey,
		"",
		"",
		"gpt-5.6-sol",
		map[int64]struct{}{cheapFailed.ID: {}},
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointResponses,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
		true,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, effectiveKey)
	require.Equal(t, &cheapGroupID, effectiveKey.GroupID)
	require.NotNil(t, selection)
	require.Equal(t, cheapAvailable.ID, selection.Account.ID)
	require.Equal(t, 0, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx,
		apiKey,
		"",
		"",
		"gpt-5.6-sol",
		map[int64]struct{}{cheapFailed.ID: {}, cheapAvailable.ID: {}},
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointResponses,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
		true,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, effectiveKey)
	require.Equal(t, &expensiveGroupID, effectiveKey.GroupID)
	require.NotNil(t, selection)
	require.Equal(t, expensiveAvailable.ID, selection.Account.ID)
	// The quality pass must not turn a request-local absence into a cross-request
	// group circuit failure. The original availability fallback owns exhaustion.
	require.Equal(t, 0, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestCandidateGroups_FiltersOpenAIAndSortsByEffectiveRate(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "claude", Platform: PlatformAnthropic, Status: StatusActive, RateMultiplier: 0.01},
			{ID: 2, Name: "openai-mid", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2, SortOrder: 20},
			{ID: 3, Name: "openai-cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 30},
			{ID: 4, Name: "openai-disabled", Platform: PlatformOpenAI, Status: StatusDisabled, RateMultiplier: 0.05},
		},
		rates: map[int64]float64{2: 0.08},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)

	got, err := resolver.CandidateGroups(context.Background(), 42, nil)

	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, groupIDsForTest(got))
}

func TestCandidateGroups_FiltersByMaxEffectiveRateMultiplier(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "cheap-default", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1},
			{ID: 2, Name: "override-cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1.5},
			{ID: 3, Name: "too-expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.9},
		},
		rates: map[int64]float64{2: 0.2},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)
	maxRate := 0.2

	got, err := resolver.CandidateGroups(context.Background(), 42, &maxRate)

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, groupIDsForTest(got))
}

func TestCandidateGroups_ZeroMaxRateMultiplierMeansUnlimited(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1},
			{ID: 2, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1.5},
		},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)
	maxRate := 0.0

	got, err := resolver.CandidateGroups(context.Background(), 42, &maxRate)

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, groupIDsForTest(got))
}

func TestCandidateGroups_TieBreaksBySortOrderThenID(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 7, Name: "g7", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 20},
			{ID: 5, Name: "g5", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 10},
			{ID: 6, Name: "g6", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 10},
		},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)

	got, err := resolver.CandidateGroups(context.Background(), 42, nil)

	require.NoError(t, err)
	require.Equal(t, []int64{5, 6, 7}, groupIDsForTest(got))
}

type fakeAvailableOpenAIGroupsProvider struct {
	groups []Group
	rates  map[int64]float64
}

func (f *fakeAvailableOpenAIGroupsProvider) GetAvailableGroups(context.Context, int64) ([]Group, error) {
	return append([]Group(nil), f.groups...), nil
}

func (f *fakeAvailableOpenAIGroupsProvider) GetUserGroupRates(context.Context, int64) (map[int64]float64, error) {
	return f.rates, nil
}

func groupIDsForTest(groups []Group) []int64 {
	out := make([]int64, 0, len(groups))
	for _, group := range groups {
		out = append(out, group.ID)
	}
	return out
}
