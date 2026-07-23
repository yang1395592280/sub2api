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

func TestOpenAIAutoCheapestSelection_PriceTierBeforeHigherGroup(t *testing.T) {
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
	svc.openAIAutoSchedulerService = NewOpenAIAutoSchedulerService(&fakeOpenAIAutoSchedulerRepo{groups: map[int64]Group{
		cheapGroupID:     {ID: cheapGroupID, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true},
		expensiveGroupID: {ID: expensiveGroupID, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: false},
	}}, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: []Group{
		{ID: cheapGroupID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
		{ID: expensiveGroupID, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2, AllowAutoCheapestScheduling: true},
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

	// A usable account in the current price tier wins before a qualified account
	// in a more expensive group.
	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, map[int64]struct{}{cheapFast.ID: {}}, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Equal(t, &cheapGroupID, effectiveKey.GroupID)
	require.Equal(t, cheapSlow.ID, selection.Account.ID)
	require.Zero(t, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	// After a first-output timeout, strict quality mode may escape to a more
	// expensive qualified account instead of retrying the slow cheap account.
	RequireOpenAIAutoCheapestQualifiedFailover(ctx)
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

	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, map[int64]struct{}{cheapFast.ID: {}, expensiveFast.ID: {}}, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.Error(t, err)
	require.Nil(t, effectiveKey)
	require.Nil(t, selection)
	require.Zero(t, circuit.calls)
}

func TestOpenAIAutoCheapestSelection_PrefersLowestChannelPriceAmongQualifiedAccounts(t *testing.T) {
	groupID := int64(113)
	observingPrice := 0.03
	cheapHealthyPrice := 0.04
	expensiveHealthyPrice := 0.05
	observing := Account{
		ID: 11301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, ChannelPrice: &observingPrice,
	}
	cheapHealthy := Account{
		ID: 11302, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, ChannelPrice: &cheapHealthyPrice,
	}
	expensiveHealthy := Account{
		ID: 11303, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, ChannelPrice: &expensiveHealthyPrice,
		Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
	}

	settings := enabledOpenAIAutoSchedulerSettings()
	settings.Mode = OpenAIAutoSchedulerModePerformance
	settings.ShadowMode = false
	settings.TopK = 1
	settings.AdaptiveTopKEnabled = false
	settings.ExplorationBudget = 0
	settings.Weights = defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModePerformance)
	cfg := newSchedulerTestSubscriptionPriorityConfig()
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{
			observing, cheapHealthy, expensiveHealthy,
		}}},
		cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{
			"openai:auto_price_session": expensiveHealthy.ID,
		}},
		cfg:              cfg,
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			observing.ID: true, cheapHealthy.ID: true, expensiveHealthy.ID: true,
		}}),
	}
	svc.openAIAutoSchedulerService = NewOpenAIAutoSchedulerService(&fakeOpenAIAutoSchedulerRepo{groups: map[int64]Group{
		groupID: {ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true},
	}}, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})
	svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: []Group{
		{ID: groupID, Name: "plus", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
	}}), nil)

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
		{account: &cheapHealthy, ttft: 800},
		{account: &expensiveHealthy, ttft: 100},
	} {
		key := svc.openAIBalancedHealthKeyForCandidate(sample.account, req)
		healthRepo.states[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: sample.ttft,
			RealSampleCount: 2, LastRealAt: &lastRealAt, ExpiresAt: now.Add(time.Hour),
		}
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

	apiKey := &APIKey{ID: 1, UserID: 9, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	ctx := PrepareOpenAIAutoCheapestRequestContext(context.Background(), true)
	effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, nil, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)

	require.NoError(t, err)
	require.Equal(t, &groupID, effectiveKey.GroupID)
	require.Equal(t, cheapHealthy.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	effectiveKey, selection, decision, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "auto_price_session", req.RequestedModel, nil, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Equal(t, &groupID, effectiveKey.GroupID)
	require.Equal(t, cheapHealthy.ID, selection.Account.ID)
	require.False(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "", "", req.RequestedModel, map[int64]struct{}{cheapHealthy.ID: {}}, req.RequiredTransport,
		req.RequiredEndpoint, OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Equal(t, &groupID, effectiveKey.GroupID)
	require.Equal(t, expensiveHealthy.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_auto_price", expensiveHealthy.ID, time.Hour))
	effectiveKey, selection, decision, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx, apiKey, "resp_auto_price", "", req.RequestedModel, nil, req.RequiredTransport, req.RequiredEndpoint,
		OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Equal(t, &groupID, effectiveKey.GroupID)
	require.Equal(t, expensiveHealthy.ID, selection.Account.ID)
	require.True(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	require.False(t, openAIAutoCheapestChannelPricePriority(context.Background()))
}

func TestOpenAIAutoCheapestImageSelection_PriceTierBeforeHigherGroup(t *testing.T) {
	cheapGroupID := int64(121)
	expensiveGroupID := int64(122)
	cheapSlow := Account{ID: 12101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID}}
	expensiveFast := Account{ID: 12201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{expensiveGroupID}}
	circuit := &autoCheapestCircuitStub{}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{cheapSlow, expensiveFast}}},
		cache:       &schedulerTestGatewayCache{}, cfg: newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			cheapSlow.ID: true, expensiveFast.ID: true,
		}}),
	}
	svc.openAIAutoSchedulerService = NewOpenAIAutoSchedulerService(&fakeOpenAIAutoSchedulerRepo{groups: map[int64]Group{
		cheapGroupID:     {ID: cheapGroupID, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true},
		expensiveGroupID: {ID: expensiveGroupID, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: false},
	}}, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: []Group{
		{ID: cheapGroupID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
		{ID: expensiveGroupID, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2, AllowAutoCheapestScheduling: true},
	}}), nil)
	svc.SetOpenAIAutoCheapestGroupCircuit(circuit)

	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-image-1", RequiredEndpoint: OpenAISchedulerEndpointImagesGen,
		RequiredTransport: OpenAIUpstreamTransportHTTPSSE, RequiredImageCapability: OpenAIImagesCapabilityBasic,
	}
	now := time.Now()
	lastRealAt := now.Add(-time.Second)
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}}
	for _, sample := range []struct {
		account *Account
		ttft    float64
	}{
		{account: &cheapSlow, ttft: 15_000},
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

	effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForImages(
		ctx, apiKey, "", req.RequestedModel, req.RequiredEndpoint, nil, req.RequiredImageCapability,
	)

	require.NoError(t, err)
	require.Equal(t, &cheapGroupID, effectiveKey.GroupID)
	require.Equal(t, cheapSlow.ID, selection.Account.ID)
	require.Zero(t, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
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
		{ID: cheapID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
		{ID: nextID, Name: "next", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2, AllowAutoCheapestScheduling: true},
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
			{ID: cheapGroupID, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
			{ID: expensiveGroupID, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.15, AllowAutoCheapestScheduling: true},
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
	// Only the availability pass may confirm exhaustion and open the group
	// circuit; the qualified pass alone never does so.
	require.Equal(t, 1, circuit.calls)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestCandidateGroups_FiltersOpenAIAndSortsByEffectiveRate(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "claude", Platform: PlatformAnthropic, Status: StatusActive, RateMultiplier: 0.01},
			{ID: 2, Name: "openai-mid", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.2, SortOrder: 20, AllowAutoCheapestScheduling: true},
			{ID: 3, Name: "openai-cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 30, AllowAutoCheapestScheduling: true},
			{ID: 4, Name: "openai-disabled", Platform: PlatformOpenAI, Status: StatusDisabled, RateMultiplier: 0.05, AllowAutoCheapestScheduling: true},
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
			{ID: 1, Name: "cheap-default", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
			{ID: 2, Name: "override-cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1.5, AllowAutoCheapestScheduling: true},
			{ID: 3, Name: "too-expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.9, AllowAutoCheapestScheduling: true},
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
			{ID: 1, Name: "cheap", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
			{ID: 2, Name: "expensive", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1.5, AllowAutoCheapestScheduling: true},
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
			{ID: 7, Name: "g7", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 20, AllowAutoCheapestScheduling: true},
			{ID: 5, Name: "g5", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 10, AllowAutoCheapestScheduling: true},
			{ID: 6, Name: "g6", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, SortOrder: 10, AllowAutoCheapestScheduling: true},
		},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)

	got, err := resolver.CandidateGroups(context.Background(), 42, nil)

	require.NoError(t, err)
	require.Equal(t, []int64{5, 6, 7}, groupIDsForTest(got))
}

func TestCandidateGroups_ExcludesGroupsThatDisableAutoCheapestScheduling(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "excluded-cheapest", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.05},
			{ID: 2, Name: "eligible", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1, AllowAutoCheapestScheduling: true},
		},
	}
	resolver := NewOpenAIAutoCheapestGroupResolver(provider)

	got, err := resolver.CandidateGroups(context.Background(), 42, nil)

	require.NoError(t, err)
	require.Equal(t, []int64{2}, groupIDsForTest(got))
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
