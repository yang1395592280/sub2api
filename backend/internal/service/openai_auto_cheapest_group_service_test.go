package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type selfHostedPoolLookupGroupRepo struct {
	GroupRepository
	groups map[int64]*Group
}

func (r selfHostedPoolLookupGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	return r.groups[id], nil
}

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

	RequireOpenAIAutoCheapestQualifiedFailover(ctx)
	effectiveKey, selection, _, err = svc.SelectEffectiveOpenAIAccountWithSchedulerForImages(
		ctx, apiKey, "", req.RequestedModel, req.RequiredEndpoint, nil, req.RequiredImageCapability,
	)
	require.NoError(t, err)
	require.Equal(t, &expensiveGroupID, effectiveKey.GroupID)
	require.Equal(t, expensiveFast.ID, selection.Account.ID)
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

func TestCandidateGroups_DynamicBillingUsesCappedMaximumForUserLimit(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{groups: []Group{
		{ID: 1, Name: "dynamic-within-limit", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, SubscriptionType: SubscriptionTypeStandard, Status: StatusActive, AllowAutoCheapestScheduling: true, DynamicBillingEnabled: true, UpstreamPriceGroupingEnabled: true, UpstreamPriceGroupingMin: 0.03, UpstreamPriceGroupingMax: 0.12},
		{ID: 2, Name: "dynamic-above-limit", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, SubscriptionType: SubscriptionTypeStandard, Status: StatusActive, AllowAutoCheapestScheduling: true, DynamicBillingEnabled: true, UpstreamPriceGroupingEnabled: true, UpstreamPriceGroupingMin: 0.13, UpstreamPriceGroupingMax: 0.22},
	}}
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)
	resolver := NewOpenAIAutoCheapestGroupResolver(provider, settings)
	maxRate := 0.15

	got, err := resolver.CandidateGroups(context.Background(), 42, &maxRate)

	require.NoError(t, err)
	require.Equal(t, []int64{1}, groupIDsForTest(got))
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

func TestCandidateGroups_ExcludesSelfHostedAccountPools(t *testing.T) {
	provider := &fakeAvailableOpenAIGroupsProvider{
		groups: []Group{
			{ID: 1, Name: "pool", Platform: PlatformOpenAI, GroupRole: GroupRoleSelfHostedPool, Status: StatusActive, RateMultiplier: 0.01, AllowAutoCheapestScheduling: true},
			{ID: 2, Name: "standard", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.15, AllowAutoCheapestScheduling: true},
		},
	}

	got, err := NewOpenAIAutoCheapestGroupResolver(provider).CandidateGroups(context.Background(), 42, nil)

	require.NoError(t, err)
	require.Equal(t, []int64{2}, groupIDsForTest(got))
}

func TestOpenAIAccountSourceGroupIDs_PoolThenEffectiveAndDisabledSkip(t *testing.T) {
	effectiveID := int64(15)
	poolID := int64(99)
	svc := &OpenAIGatewayService{}
	effective := &Group{
		ID: effectiveID, Platform: PlatformOpenAI, GroupRole: GroupRoleStandard,
		SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive,
	}

	sources := svc.openAIAccountSourceGroupIDs(context.Background(), &effectiveID, effective)
	require.Len(t, sources, 2)
	require.Equal(t, poolID, *sources[0])
	require.Equal(t, effectiveID, *sources[1])

	effective.SelfHostedPoolStatus = StatusDisabled
	sources = svc.openAIAccountSourceGroupIDs(context.Background(), &effectiveID, effective)
	require.Len(t, sources, 1)
	require.Equal(t, effectiveID, *sources[0])
}

func TestOpenAIAccountSourceGroupIDs_FixedGroupAfterAuthSnapshotRoundTrip(t *testing.T) {
	effectiveID := int64(15)
	poolID := int64(99)
	authService := &APIKeyService{}
	snapshot := authService.snapshotFromAPIKey(context.Background(), &APIKey{
		ID: 1, UserID: 2, GroupID: &effectiveID, Status: StatusActive,
		User: &User{ID: 2, Status: StatusActive, Role: RoleUser},
		Group: &Group{
			ID: effectiveID, Platform: PlatformOpenAI, GroupRole: GroupRoleStandard,
			Status: StatusActive, SelfHostedPoolGroupID: &poolID,
		},
	})
	restored := authService.snapshotToAPIKey("k-fixed", snapshot)
	require.NotNil(t, restored)
	require.NotNil(t, restored.Group)

	groupRepo := selfHostedPoolLookupGroupRepo{groups: map[int64]*Group{
		poolID: {ID: poolID, Platform: PlatformOpenAI, GroupRole: GroupRoleSelfHostedPool, Status: StatusActive},
	}}
	gateway := &OpenAIGatewayService{schedulerSnapshot: NewSchedulerSnapshotService(nil, nil, nil, groupRepo, nil)}
	sources := gateway.openAIAccountSourceGroupIDs(context.Background(), restored.GroupID, restored.Group)
	require.Len(t, sources, 2)
	require.Equal(t, poolID, *sources[0])
	require.Equal(t, effectiveID, *sources[1])

	groupRepo.groups[poolID].Status = StatusDisabled
	sources = gateway.openAIAccountSourceGroupIDs(context.Background(), restored.GroupID, restored.Group)
	require.Len(t, sources, 1)
	require.Equal(t, effectiveID, *sources[0])
}

func TestAutoCheapestSharedPoolStageOrder(t *testing.T) {
	poolID := int64(99)
	groups := []Group{
		{ID: 10, Name: "0.10", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.10, AllowAutoCheapestScheduling: true},
		{ID: 15, Name: "0.15", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.15, AllowAutoCheapestScheduling: true, SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive},
		{ID: 20, Name: "0.20", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.20, AllowAutoCheapestScheduling: true, SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive},
	}
	candidates, err := NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: groups}).CandidateGroups(context.Background(), 42, nil)
	require.NoError(t, err)
	apiKey := &APIKey{ID: 1, UserID: 42, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
	keys := make([]*APIKey, 0, len(candidates))
	for i := range candidates {
		keys = append(keys, CloneAPIKeyForEffectiveGroup(apiKey, &candidates[i]))
	}

	var stages []int64
	var effectiveGroupIDs []int64
	var auditedPoolIDs []int64
	var fallbackReasons []string
	svc := &OpenAIGatewayService{}
	for _, stage := range svc.openAIAutoCheapestAccountSourceStages(context.Background(), keys) {
		stageCtx := openAIAutoCheapestStageContext(context.Background(), stage)
		sourceID := openAIAccountSourceGroupFromContext(stageCtx, stage.effectiveKey.GroupID)
		stages = append(stages, *sourceID)
		effectiveGroupIDs = append(effectiveGroupIDs, *stage.effectiveKey.GroupID)
		metadata := openAIPoolStageMetadataFromContext(stageCtx)
		auditedPoolIDs = append(auditedPoolIDs, metadata.PoolGroupID)
		fallbackReasons = append(fallbackReasons, metadata.FallbackReason)
	}

	require.Equal(t, []int64{99, 10, 15, 20}, stages)
	require.Equal(t, []int64{15, 10, 15, 20}, effectiveGroupIDs)
	require.Equal(t, []int64{99, 0, 99, 99}, auditedPoolIDs)
	require.Equal(t, []string{"", "", "no_available_accounts", "no_available_accounts"}, fallbackReasons)
}

func TestOpenAIAutoCheapestSelection_PrefersSelfHostedPoolBeforeCheaperBusinessGroup(t *testing.T) {
	cheapGroupID := int64(10)
	pooledGroupID := int64(15)
	poolID := int64(99)
	cheapUpstream := Account{
		ID: 1001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 1, GroupIDs: []int64{cheapGroupID},
	}
	selfHosted := Account{
		ID: 9901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 1, GroupIDs: []int64{poolID},
	}
	newService := func() *OpenAIGatewayService {
		svc := &OpenAIGatewayService{
			accountRepo:      schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{cheapUpstream, selfHosted}}},
			cache:            &schedulerTestGatewayCache{},
			cfg:              newSchedulerTestSubscriptionPriorityConfig(),
			rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
			concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
				cheapUpstream.ID: true,
				selfHosted.ID:    true,
			}}),
		}
		svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{groups: []Group{
			{ID: cheapGroupID, Name: "cheap-upstream", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.10, AllowAutoCheapestScheduling: true},
			{ID: pooledGroupID, Name: "self-hosted-first", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, Status: StatusActive, RateMultiplier: 0.15, AllowAutoCheapestScheduling: true, SelfHostedPoolGroupID: &poolID, SelfHostedPoolStatus: StatusActive},
		}}), nil)
		return svc
	}
	apiKey := &APIKey{ID: 1, UserID: 42, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}

	t.Run("load awareness", func(t *testing.T) {
		svc := newService()
		effectiveKey, selection, err := svc.SelectEffectiveOpenAIAccountWithLoadAwareness(
			context.Background(), apiKey, "", "gpt-5.6-sol", nil, OpenAIEndpointCapabilityChatCompletions,
		)
		require.NoError(t, err)
		require.Equal(t, &pooledGroupID, effectiveKey.GroupID)
		require.Equal(t, selfHosted.ID, selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})

	t.Run("advanced scheduler", func(t *testing.T) {
		svc := newService()
		effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
			context.Background(), apiKey, "", "", "gpt-5.6-sol", nil,
			OpenAIUpstreamTransportHTTPSSE, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.Equal(t, &pooledGroupID, effectiveKey.GroupID)
		require.Equal(t, selfHosted.ID, selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})

	t.Run("images scheduler", func(t *testing.T) {
		svc := newService()
		effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForImages(
			context.Background(), apiKey, "", "gpt-image-1", OpenAISchedulerEndpointImagesGen,
			nil, OpenAIImagesCapabilityBasic,
		)
		require.NoError(t, err)
		require.Equal(t, &pooledGroupID, effectiveKey.GroupID)
		require.Equal(t, selfHosted.ID, selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})

	t.Run("pool exhausted falls back by cheapest group", func(t *testing.T) {
		svc := newService()
		effectiveKey, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
			context.Background(), apiKey, "", "", "gpt-5.6-sol", map[int64]struct{}{selfHosted.ID: {}},
			OpenAIUpstreamTransportHTTPSSE, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.Equal(t, &cheapGroupID, effectiveKey.GroupID)
		require.Equal(t, cheapUpstream.ID, selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})
}

func TestSelectAccountWithLoadAwareness_UsesSourceMembershipAndEffectivePriceGuard(t *testing.T) {
	effectiveID := int64(15)
	poolID := int64(99)
	poolPriority := 1
	groupPriority := 1
	poolPrice := 0.2
	groupPrice := 0.1
	effective := &Group{ID: effectiveID, Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, UpstreamPriceMaxMultiplier: 0.15}
	poolAccount := Account{
		ID: 9901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
		ChannelPrice: &poolPrice, GroupIDs: []int64{poolID}, AccountGroups: []AccountGroup{{GroupID: poolID, Priority: poolPriority}},
	}
	groupAccount := Account{
		ID: 1501, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
		ChannelPrice: &groupPrice, GroupIDs: []int64{effectiveID}, AccountGroups: []AccountGroup{{GroupID: effectiveID, Priority: groupPriority}},
	}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{poolAccount, groupAccount}}},
		cfg:         newSchedulerTestSubscriptionPriorityConfig(),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			poolAccount.ID: true, groupAccount.ID: true,
		}}),
	}
	ctx := withOpenAIEffectiveGroup(context.Background(), effective)
	ctx = withOpenAIAccountSourceGroup(ctx, &poolID)

	selection, err := svc.selectAccountWithLoadAwareness(ctx, &effectiveID, PlatformOpenAI, "", "gpt-5.6-sol", nil, false, "", false)

	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)

	effective.UpstreamPriceMaxMultiplier = 0.25
	selection, err = svc.selectAccountWithLoadAwareness(ctx, &effectiveID, PlatformOpenAI, "", "gpt-5.6-sol", nil, false, "", false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, poolAccount.ID, selection.Account.ID)
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
