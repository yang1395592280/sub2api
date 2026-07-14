package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAISnapshotCacheStub struct {
	SchedulerCache
	snapshotAccounts []*Account
	accountsByID     map[int64]*Account
}

type schedulerTestOpenAIAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r schedulerTestOpenAIAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, errors.New("account not found")
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	platformSet := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		platformSet[platform] = struct{}{}
	}
	var result []Account
	for _, acc := range r.accounts {
		if _, ok := platformSet[acc.Platform]; ok {
			result = append(result, acc)
		}
	}
	return result, nil
}

type schedulerGroupAwareOpenAIAccountRepo struct {
	schedulerTestOpenAIAccountRepo
}

func (r schedulerGroupAwareOpenAIAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform && openAIStickyAccountMatchesGroup(&acc, &groupID) {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r schedulerGroupAwareOpenAIAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform && openAIStickyAccountMatchesGroup(&acc, nil) {
			result = append(result, acc)
		}
	}
	return result, nil
}

type schedulerTestConcurrencyCache struct {
	ConcurrencyCache
	loadBatchErr    error
	loadMap         map[int64]*AccountLoadInfo
	acquireResults  map[int64]bool
	acquireOrder    *[]int64
	waitCounts      map[int64]int
	skipDefaultLoad bool
}

func (c schedulerTestConcurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	if c.acquireOrder != nil {
		*c.acquireOrder = append(*c.acquireOrder, accountID)
	}
	if c.acquireResults != nil {
		if result, ok := c.acquireResults[accountID]; ok {
			return result, nil
		}
	}
	return true, nil
}

func (c schedulerTestConcurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	return nil
}

func (c schedulerTestConcurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	if c.loadBatchErr != nil {
		return nil, c.loadBatchErr
	}
	out := make(map[int64]*AccountLoadInfo, len(accounts))
	if c.skipDefaultLoad && c.loadMap != nil {
		for _, acc := range accounts {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
			}
		}
		return out, nil
	}
	for _, acc := range accounts {
		if c.loadMap != nil {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
				continue
			}
		}
		out[acc.ID] = &AccountLoadInfo{AccountID: acc.ID, LoadRate: 0}
	}
	return out, nil
}

func (c schedulerTestConcurrencyCache) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if c.waitCounts != nil {
		if count, ok := c.waitCounts[accountID]; ok {
			return count, nil
		}
	}
	return 0, nil
}

type schedulerTestGatewayCache struct {
	sessionBindings map[string]int64
	deletedSessions map[string]int
}

func (c *schedulerTestGatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if id, ok := c.sessionBindings[sessionHash]; ok {
		return id, nil
	}
	return 0, errors.New("not found")
}

func (c *schedulerTestGatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if c.sessionBindings == nil {
		c.sessionBindings = make(map[string]int64)
	}
	c.sessionBindings[sessionHash] = accountID
	return nil
}

func (c *schedulerTestGatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (c *schedulerTestGatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if c.sessionBindings == nil {
		return nil
	}
	if c.deletedSessions == nil {
		c.deletedSessions = make(map[string]int)
	}
	c.deletedSessions[sessionHash]++
	delete(c.sessionBindings, sessionHash)
	return nil
}

func newSchedulerTestOpenAIWSV2Config() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	return cfg
}

func newSchedulerTestSubscriptionPriorityConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0
	return cfg
}

type openAIAdvancedSchedulerSettingRepoStub struct {
	values map[string]string
}

func (s *openAIAdvancedSchedulerSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value}, nil
}

func (s *openAIAdvancedSchedulerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s == nil || s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *openAIAdvancedSchedulerSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected call to Set")
}

func (s *openAIAdvancedSchedulerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, err := s.GetValue(context.Background(), key); err == nil {
			result[key] = value
		}
	}
	return result, nil
}

func (s *openAIAdvancedSchedulerSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected call to SetMultiple")
}

func (s *openAIAdvancedSchedulerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected call to GetAll")
}

func (s *openAIAdvancedSchedulerSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected call to Delete")
}

func newOpenAIAdvancedSchedulerRateLimitService(enabled string, values ...string) *RateLimitService {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	repo := &openAIAdvancedSchedulerSettingRepoStub{
		values: map[string]string{},
	}
	if enabled != "" {
		repo.values[openAIAdvancedSchedulerSettingKey] = enabled
	}
	if len(values) > 0 && values[0] != "" {
		repo.values[SettingKeyOpenAIAdvancedSchedulerStickyWeightedEnabled] = values[0]
	}
	if len(values) > 1 && values[1] != "" {
		repo.values[SettingKeyOpenAIAdvancedSchedulerSubscriptionPriorityEnabled] = values[1]
	}
	return &RateLimitService{
		settingService: NewSettingService(repo, &config.Config{}),
	}
}

func (s *openAISnapshotCacheStub) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	if len(s.snapshotAccounts) == 0 {
		return nil, false, nil
	}
	out := make([]*Account, 0, len(s.snapshotAccounts))
	for _, account := range s.snapshotAccounts {
		if account == nil {
			continue
		}
		cloned := *account
		out = append(out, &cloned)
	}
	return out, true, nil
}

func (s *openAISnapshotCacheStub) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s.accountsByID == nil {
		return nil, nil
	}
	account := s.accountsByID[accountID]
	if account == nil {
		return nil, nil
	}
	cloned := *account
	return &cloned, nil
}

func TestOpenAIGatewayService_OpenAIAdvancedSchedulerRuntimeSettings_DBOverridesConfig(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 11
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights = config.GatewayOpenAIWSSchedulerScoreWeights{
		Priority:         1,
		Load:             2,
		Queue:            3,
		ErrorRate:        4,
		TTFT:             5,
		Reset:            6,
		QuotaHeadroom:    7,
		PreviousResponse: 8,
		SessionSticky:    9,
	}
	repo := &openAIAdvancedSchedulerSettingRepoStub{
		values: map[string]string{
			openAIAdvancedSchedulerSettingKey:                       "true",
			SettingKeyOpenAIAdvancedSchedulerLBTopK:                 "3",
			SettingKeyOpenAIAdvancedSchedulerWeightPriority:         "2.5",
			SettingKeyOpenAIAdvancedSchedulerWeightPreviousResponse: "12",
		},
	}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		rateLimitService: &RateLimitService{settingService: NewSettingService(repo, cfg)},
	}

	ctx := context.Background()
	require.Equal(t, 3, svc.openAIWSLBTopKForRequest(ctx))
	weights := svc.openAIWSSchedulerWeightsForRequest(ctx)
	require.Equal(t, 2.5, weights.Priority)
	require.Equal(t, 2.0, weights.Load)
	require.Equal(t, 12.0, weights.Previous)
	require.Equal(t, 9.0, weights.SessionSticky)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_DefaultDisabledUsesLegacyLoadAwareness(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10106)
	accounts := []Account{
		{
			ID:          36001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
		},
		{
			ID:          36002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cache := &schedulerTestGatewayCache{}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_disabled_001", 36001, time.Hour))
	require.False(t, svc.isOpenAIAdvancedSchedulerEnabled(ctx))

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"resp_disabled_001",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(36002), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickyPreviousHit)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_DefaultDisabled_RequiredWSV2_SkipsHTTPOnlyAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10108)
	accounts := []Account{
		{
			ID:          36011,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
		{
			ID:          36012,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportResponsesWebsocketV2,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(36012), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_DefaultDisabled_RequiredWSV2_NoAvailableAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10109)
	accounts := []Account{
		{
			ID:          36021,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportResponsesWebsocketV2,
		false,
	)
	require.ErrorContains(t, err, "no available OpenAI accounts")
	require.Nil(t, selection)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_DefaultDisabled_EmbeddingsSkipsChatOnlyAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10110)
	accounts := []Account{
		{
			ID:          36031,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions"},
			},
		},
		{
			ID:          36032,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions", "embeddings"},
			},
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"",
		"",
		"text-embedding-3-small",
		nil,
		OpenAIUpstreamTransportHTTPSSE,
		OpenAISchedulerEndpointEmbeddings,
		OpenAIEndpointCapabilityEmbeddings,
		false,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(36032), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_DefaultDisabled_AllowsGrokChatAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10113)
	accounts := []Account{
		{
			ID:          36041,
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"",
		"",
		"grok-4.3",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointChatCompletions,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
		PlatformGrok,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(36041), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_EnabledUsesAdvancedPreviousResponseRouting(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10107)
	accounts := []Account{
		{
			ID:          37001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
		{
			ID:          37002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_enabled_001", 37001, time.Hour))
	require.True(t, svc.isOpenAIAdvancedSchedulerEnabled(ctx))

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"resp_enabled_001",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37001), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
	require.True(t, decision.StickyPreviousHit)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_StickyWeightedSessionInTopKUsesStickyFirst(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101071)
	accounts := []Account{
		{
			ID:          37101,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    100,
			GroupIDs:    []int64{groupID},
		},
		{
			ID:          37102,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.SessionSticky = 3
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{
		"openai:session_hash_weighted_topk": 37101,
	}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_weighted_topk",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37101), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.True(t, decision.StickySessionHit)
	require.Equal(t, 2, decision.TopK)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_StickyWeightedPreviousRequiresMovableContext(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101072)
	accounts := []Account{
		{
			ID:          37111,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    100,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
		{
			ID:          37112,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_weighted_unmovable", 37111, time.Hour))

	selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"resp_weighted_unmovable",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointChatCompletions,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
		PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37111), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
	require.True(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	selection, decision, err = svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"resp_weighted_unmovable",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointChatCompletions,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		true,
		PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37112), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectEffectiveAccountForwardsPreviousResponseCanMoveHint(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101073)
	accounts := []Account{
		{
			ID:          37113,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    100,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
		{
			ID:          37114,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_effective_move", 37113, time.Hour))

	apiKey := &APIKey{ID: 1, UserID: 1, GroupID: &groupID}
	effectiveKey, selection, decision, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
		ctx,
		apiKey,
		"resp_effective_move",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAISchedulerEndpointResponses,
		OpenAIEndpointCapabilityChatCompletions,
		false,
		true,
		PlatformOpenAI,
	)
	require.NoError(t, err)
	require.Same(t, apiKey, effectiveKey)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37114), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectEffectiveAccountUsesMappedModelForBalancedHealthKey(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	groupID := int64(101074)
	account := Account{
		ID:          37115,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"channel-model": "upstream-model"},
		},
	}

	tests := []struct {
		name             string
		endpoint         string
		transport        OpenAIUpstreamTransport
		capability       OpenAIEndpointCapability
		imageCapability  OpenAIImagesCapability
		useImagesWrapper bool
	}{
		{name: "responses", endpoint: OpenAISchedulerEndpointResponses, transport: OpenAIUpstreamTransportHTTPSSE, capability: OpenAIEndpointCapabilityChatCompletions},
		{name: "chat", endpoint: OpenAISchedulerEndpointChatCompletions, transport: OpenAIUpstreamTransportHTTPSSE, capability: OpenAIEndpointCapabilityChatCompletions},
		{name: "embeddings", endpoint: OpenAISchedulerEndpointEmbeddings, transport: OpenAIUpstreamTransportHTTPSSE, capability: OpenAIEndpointCapabilityEmbeddings},
		{name: "images", endpoint: OpenAISchedulerEndpointImagesGen, transport: OpenAIUpstreamTransportHTTPSSE, imageCapability: OpenAIImagesCapabilityBasic, useImagesWrapper: true},
		{name: "websocket", endpoint: OpenAISchedulerEndpointResponses, transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress, capability: OpenAIEndpointCapabilityChatCompletions},
	}

	for _, mode := range []string{"fixed", "auto_effective_group"} {
		for _, tt := range tests {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				healthRepo := &balancedSchedulerHealthRepoStub{}
				cfg := newSchedulerTestSubscriptionPriorityConfig()
				cfg.Gateway.OpenAIWS.Enabled = true
				cfg.Gateway.OpenAIWS.APIKeyEnabled = true
				cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
				svc := &OpenAIGatewayService{
					accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{account}}},
					cache:              &schedulerTestGatewayCache{},
					cfg:                cfg,
					rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
				}
				svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

				apiKey := &APIKey{ID: 1, UserID: 9, GroupID: &groupID}
				if mode == "auto_effective_group" {
					apiKey = &APIKey{ID: 1, UserID: 9, GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}
					svc.SetOpenAIAutoCheapestGroupResolver(NewOpenAIAutoCheapestGroupResolver(&fakeAvailableOpenAIGroupsProvider{
						groups: []Group{{ID: groupID, Name: "effective", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.1}},
					}), nil)
				}
				modelResolver := func(candidate *APIKey, model string) string {
					require.NotNil(t, candidate)
					require.Equal(t, &groupID, candidate.GroupID)
					require.Equal(t, "client-model", model)
					return "channel-model"
				}

				if tt.useImagesWrapper {
					_, _, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForImagesAndModelResolver(
						context.Background(), apiKey, "", "client-model", tt.endpoint, nil, tt.imageCapability, modelResolver,
					)
					require.NoError(t, err)
					require.NotNil(t, selection)
				} else {
					_, _, selection, _, err := svc.SelectEffectiveOpenAIAccountWithSchedulerForCapabilityAndModelResolver(
						context.Background(), apiKey, "", "", "client-model", nil, tt.transport, tt.endpoint,
						tt.capability, false, false, PlatformOpenAI, modelResolver,
					)
					require.NoError(t, err)
					require.NotNil(t, selection)
				}

				require.NotEmpty(t, healthRepo.keys)
				for _, key := range healthRepo.keys {
					require.Equal(t, "upstream-model", key.ModelFamily)
				}
			})
		}
	}
}

func TestOpenAIGatewayService_SelectAccountBalancedHealthUsesActualWSIngressTransport(t *testing.T) {
	groupID := int64(101075)

	tests := []struct {
		name              string
		modeRouterEnabled bool
		accountMode       string
		wantTransport     OpenAIUpstreamTransport
		wantHealthLoad    bool
	}{
		{name: "mode router passthrough", modeRouterEnabled: true, accountMode: OpenAIWSIngressModePassthrough, wantTransport: OpenAIUpstreamTransportResponsesWebsocketV2, wantHealthLoad: true},
		{name: "mode router http bridge", modeRouterEnabled: true, accountMode: OpenAIWSIngressModeHTTPBridge, wantTransport: OpenAIUpstreamTransportHTTPSSE, wantHealthLoad: true},
		{name: "mode router ctx pool falls back for whole request", modeRouterEnabled: true, accountMode: OpenAIWSIngressModeCtxPool, wantHealthLoad: false},
		{name: "legacy router uses protocol resolver", modeRouterEnabled: false, wantTransport: OpenAIUpstreamTransportResponsesWebsocketV2, wantHealthLoad: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			account := Account{
				ID:          37116,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				GroupIDs:    []int64{groupID},
				Extra: map[string]any{
					"openai_apikey_responses_websockets_v2_enabled": true,
				},
			}
			if tt.accountMode != "" {
				account.Extra["openai_apikey_responses_websockets_v2_mode"] = tt.accountMode
			}
			cfg := newSchedulerTestOpenAIWSV2Config()
			cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = tt.modeRouterEnabled
			cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
			healthRepo := &balancedSchedulerHealthRepoStub{}
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{account}}},
				cache:              &schedulerTestGatewayCache{},
				cfg:                cfg,
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

			selection, _, err := svc.SelectAccountWithSchedulerForCapability(
				context.Background(), &groupID, "", "", "gpt-5.4", nil,
				OpenAIUpstreamTransportResponsesWebsocketV2Ingress,
				OpenAISchedulerEndpointResponses,
				OpenAIEndpointCapabilityChatCompletions,
				false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)

			if !tt.wantHealthLoad {
				require.Zero(t, healthRepo.getCalls)
				require.Empty(t, healthRepo.keys)
				return
			}
			require.Equal(t, 1, healthRepo.getCalls)
			require.NotEmpty(t, healthRepo.keys)
			for _, key := range healthRepo.keys {
				require.Equal(t, string(tt.wantTransport), key.Transport)
			}
		})
	}
}

func TestOpenAIGatewayService_SelectAccountBalancedDoesNotReappendCircuitRejectedCandidates(t *testing.T) {
	groupID := int64(101076)

	for _, rejectedState := range []string{OpenAIAutoSchedulerStateOpen, OpenAIAutoSchedulerStateHalfOpen} {
		t.Run(rejectedState, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			rejected := Account{ID: 37117, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
			running := Account{ID: 37118, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID}}
			healthKeyService := &OpenAIGatewayService{}
			req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
			rejectedKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&rejected, req)
			runningKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&running, req)
			now := time.Now()
			healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
				rejectedKey: {Key: rejectedKey, State: rejectedState, PredictedTTFTMS: 100, ExpiresAt: now.Add(time.Hour)},
				runningKey:  {Key: runningKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Hour)},
			}}
			acquireOrder := []int64{}
			svc := &OpenAIGatewayService{
				accountRepo:      schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{rejected, running}}},
				cache:            &schedulerTestGatewayCache{},
				cfg:              newSchedulerTestSubscriptionPriorityConfig(),
				rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
					acquireOrder: &acquireOrder,
					acquireResults: map[int64]bool{
						rejected.ID: true,
						running.ID:  false,
					},
				}),
			}
			svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

			selection, _, err := svc.SelectAccountWithSchedulerForCapability(
				context.Background(), &groupID, "", "", "gpt-5.4", nil,
				OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
				OpenAIEndpointCapabilityChatCompletions, false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.False(t, selection.Acquired)
			require.NotNil(t, selection.WaitPlan)
			require.Equal(t, running.ID, selection.WaitPlan.AccountID)
			require.NotContains(t, acquireOrder, rejected.ID)
		})
	}
}

func TestOpenAIGatewayService_SelectAccountBalancedKeepsRunningLatencyTailForFailover(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(101077)
	fast := Account{ID: 37119, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
	slow := Account{ID: 37120, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID}}
	healthKeyService := &OpenAIGatewayService{}
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
	fastKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&fast, req)
	slowKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&slow, req)
	now := time.Now()
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		fastKey: {Key: fastKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 100, ExpiresAt: now.Add(time.Hour)},
		slowKey: {Key: slowKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 5000, ExpiresAt: now.Add(time.Hour)},
	}}
	acquireOrder := []int64{}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{fast, slow}}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireOrder: &acquireOrder,
			acquireResults: map[int64]bool{
				fast.ID: false,
				slow.ID: true,
			},
		}),
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

	selection, _, err := svc.SelectAccountWithSchedulerForCapability(
		context.Background(), &groupID, "", "", "gpt-5.4", nil,
		OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
		OpenAIEndpointCapabilityChatCompletions, false, false, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, selection.Acquired)
	require.Equal(t, slow.ID, selection.Account.ID)
	require.Equal(t, []int64{fast.ID, slow.ID}, acquireOrder)
}

func TestOpenAIGatewayService_SelectAccountBalancedCircuitRejectedStickyBoundaries(t *testing.T) {
	groupID := int64(101078)
	sticky := Account{
		ID: 37121, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
	}
	running := Account{
		ID: 37122, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID},
		Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
	}
	newService := func(t *testing.T, stickyWeighted bool, acquireResults map[int64]bool) (*OpenAIGatewayService, *balancedSchedulerHealthRepoStub, *[]int64) {
		t.Helper()
		req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
		healthKeyService := &OpenAIGatewayService{}
		stickyKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&sticky, req)
		runningKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&running, req)
		now := time.Now()
		healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			stickyKey:  {Key: stickyKey, State: OpenAIAutoSchedulerStateOpen, PredictedTTFTMS: 100, ExpiresAt: now.Add(time.Hour)},
			runningKey: {Key: runningKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Hour)},
		}}
		acquireOrder := []int64{}
		stickyWeightedSetting := "false"
		if stickyWeighted {
			stickyWeightedSetting = "true"
		}
		cfg := newSchedulerTestSubscriptionPriorityConfig()
		cfg.Gateway.OpenAIWS.Enabled = true
		cfg.Gateway.OpenAIWS.APIKeyEnabled = true
		cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
		cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
		svc := &OpenAIGatewayService{
			accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{sticky, running}}},
			cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{
				"openai:sticky-circuit": sticky.ID,
			}},
			cfg:              cfg,
			rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", stickyWeightedSetting),
			concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
				acquireOrder:   &acquireOrder,
				acquireResults: acquireResults,
			}),
		}
		svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))
		return svc, healthRepo, &acquireOrder
	}

	t.Run("ordinary session sticky yields to running account", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t, false, map[int64]bool{sticky.ID: true, running.ID: true})

		selection, _, err := svc.SelectAccountWithScheduler(
			context.Background(), &groupID, "", "sticky-circuit", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.True(t, selection.Acquired)
		require.Equal(t, running.ID, selection.Account.ID)
		require.NotContains(t, *acquireOrder, sticky.ID)
		require.Equal(t, 1, healthRepo.getCalls)
	})

	t.Run("weighted sticky fallback does not reacquire rejected account", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t, true, map[int64]bool{sticky.ID: true, running.ID: false})

		selection, _, err := svc.SelectAccountWithScheduler(
			context.Background(), &groupID, "", "sticky-circuit", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.False(t, selection.Acquired)
		require.NotNil(t, selection.WaitPlan)
		require.Equal(t, running.ID, selection.WaitPlan.AccountID)
		require.NotContains(t, *acquireOrder, sticky.ID)
		require.Equal(t, 1, healthRepo.getCalls)
	})

	t.Run("movable previous response yields to running account", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t, false, map[int64]bool{sticky.ID: true, running.ID: true})
		store := svc.getOpenAIWSStateStore()
		require.NoError(t, store.BindResponseAccount(context.Background(), groupID, "resp-circuit-movable", sticky.ID, time.Hour))

		selection, _, err := svc.SelectAccountWithSchedulerForCapability(
			context.Background(), &groupID, "resp-circuit-movable", "", "gpt-5.4", nil,
			OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, true, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.True(t, selection.Acquired)
		require.Equal(t, running.ID, selection.Account.ID)
		require.NotContains(t, *acquireOrder, sticky.ID)
		require.Equal(t, 1, healthRepo.getCalls)
	})

	t.Run("non movable previous response remains strong", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t, false, map[int64]bool{sticky.ID: true, running.ID: true})
		store := svc.getOpenAIWSStateStore()
		require.NoError(t, store.BindResponseAccount(context.Background(), groupID, "resp-circuit-strong", sticky.ID, time.Hour))

		selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
			context.Background(), &groupID, "resp-circuit-strong", "", "gpt-5.4", nil,
			OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, false, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.True(t, selection.Acquired)
		require.Equal(t, sticky.ID, selection.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
		require.Equal(t, []int64{sticky.ID}, *acquireOrder)
		require.Zero(t, healthRepo.getCalls)
	})

	t.Run("health load failure keeps ordinary session sticky", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t, false, map[int64]bool{sticky.ID: true, running.ID: true})
		healthRepo.err = errors.New("health unavailable")

		selection, decision, err := svc.SelectAccountWithScheduler(
			context.Background(), &groupID, "", "sticky-circuit", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.True(t, selection.Acquired)
		require.Equal(t, sticky.ID, selection.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
		require.Equal(t, []int64{sticky.ID}, *acquireOrder)
		require.Equal(t, 1, healthRepo.getCalls)
	})
}

func TestOpenAIGatewayService_SelectAccountBalancedSoftStickyUsesUnifiedTTFTEscape(t *testing.T) {
	groupID := int64(101079)
	sticky := Account{
		ID: 37131, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
	}
	fast := Account{
		ID: 37132, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID},
		Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
	}
	newService := func(t *testing.T) (*OpenAIGatewayService, *balancedSchedulerHealthRepoStub, *[]int64) {
		t.Helper()
		req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
		keyService := &OpenAIGatewayService{}
		stickyKey := keyService.openAIBalancedHealthKeyForCandidate(&sticky, req)
		fastKey := keyService.openAIBalancedHealthKeyForCandidate(&fast, req)
		now := time.Now()
		healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			stickyKey: {Key: stickyKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 2600, ExpiresAt: now.Add(time.Hour)},
			fastKey:   {Key: fastKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 1200, ExpiresAt: now.Add(time.Hour)},
		}}
		acquireOrder := []int64{}
		cfg := newSchedulerTestSubscriptionPriorityConfig()
		cfg.Gateway.OpenAIWS.Enabled = true
		cfg.Gateway.OpenAIWS.APIKeyEnabled = true
		cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
		svc := &OpenAIGatewayService{
			accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{sticky, fast}}},
			cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{
				"openai:soft-sticky-ttft": sticky.ID,
			}},
			cfg:                cfg,
			rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "false"),
			openaiAccountStats: newOpenAIAccountRuntimeStats(),
			concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireOrder: &acquireOrder}),
		}
		svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))
		return svc, healthRepo, &acquireOrder
	}

	t.Run("ordinary session yields to faster unified-health candidate", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t)
		selection, decision, err := svc.SelectAccountWithScheduler(
			context.Background(), &groupID, "", "soft-sticky-ttft", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.Equal(t, fast.ID, selection.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
		require.NotContains(t, *acquireOrder, sticky.ID)
		require.Equal(t, 1, healthRepo.getCalls)
	})

	t.Run("movable previous yields to faster unified-health candidate", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t)
		store := svc.getOpenAIWSStateStore()
		require.NoError(t, store.BindResponseAccount(context.Background(), groupID, "resp-soft-sticky-ttft", sticky.ID, time.Hour))

		selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
			context.Background(), &groupID, "resp-soft-sticky-ttft", "", "gpt-5.4", nil,
			OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, true, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.Equal(t, fast.ID, selection.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
		require.NotContains(t, *acquireOrder, sticky.ID)
		require.Equal(t, 1, healthRepo.getCalls)
	})

	t.Run("non movable previous remains strong", func(t *testing.T) {
		svc, healthRepo, acquireOrder := newService(t)
		store := svc.getOpenAIWSStateStore()
		require.NoError(t, store.BindResponseAccount(context.Background(), groupID, "resp-strong-slow", sticky.ID, time.Hour))

		selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
			context.Background(), &groupID, "resp-strong-slow", "", "gpt-5.4", nil,
			OpenAIUpstreamTransportAny, OpenAISchedulerEndpointResponses,
			OpenAIEndpointCapabilityChatCompletions, false, false, PlatformOpenAI,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.Equal(t, sticky.ID, selection.Account.ID)
		require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
		require.Equal(t, []int64{sticky.ID}, *acquireOrder)
		require.Zero(t, healthRepo.getCalls)
	})

	for _, tt := range []struct {
		name            string
		waitingCount    int
		rateLimitedRate float64
		serverErrorRate float64
		errorRate       float64
	}{
		{name: "queue", waitingCount: 1},
		{name: "rate limited", rateLimitedRate: 0.2},
		{name: "server error", serverErrorRate: 0.2},
		{name: "error rate", errorRate: 0.6},
	} {
		t.Run("ordinary session hard escape on "+tt.name, func(t *testing.T) {
			svc, healthRepo, acquireOrder := newService(t)
			for key, snapshot := range healthRepo.states {
				snapshot.PredictedTTFTMS = 1200
				if key.AccountID == sticky.ID {
					snapshot.RateLimitedRate = tt.rateLimitedRate
					snapshot.ServerErrorRate = tt.serverErrorRate
					snapshot.ErrorRate = tt.errorRate
				}
				healthRepo.states[key] = snapshot
			}
			svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{
				acquireOrder: acquireOrder,
				loadMap: map[int64]*AccountLoadInfo{
					sticky.ID: {AccountID: sticky.ID, WaitingCount: tt.waitingCount},
					fast.ID:   {AccountID: fast.ID},
				},
			})

			selection, decision, err := svc.SelectAccountWithScheduler(
				context.Background(), &groupID, "", "soft-sticky-ttft", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, fast.ID, selection.Account.ID)
			require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
			require.NotContains(t, *acquireOrder, sticky.ID)
			require.Equal(t, 1, healthRepo.getCalls)
		})
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_PreviousResponseCompactUnsupportedDeletesBinding(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101073)
	accounts := []Account{
		{
			ID:          37121,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
				"openai_compact_mode":                           OpenAICompactModeForceOff,
			},
		},
		{
			ID:          37122,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    10,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
				"openai_compact_mode":                           OpenAICompactModeForceOn,
			},
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.OpenAIWS.LBTopK = 2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_compact_unsupported", 37121, time.Hour))

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"resp_compact_unsupported",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37122), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickyPreviousHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	accountID, err := store.GetResponseAccount(ctx, groupID, "resp_compact_unsupported")
	require.NoError(t, err)
	require.Zero(t, accountID)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_Enabled_EmbeddingsSkipsChatOnlyAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10111)
	accounts := []Account{
		{
			ID:          37011,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions"},
			},
		},
		{
			ID:          37012,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions", "embeddings"},
			},
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"",
		"",
		"text-embedding-3-small",
		nil,
		OpenAIUpstreamTransportHTTPSSE,
		OpenAISchedulerEndpointEmbeddings,
		OpenAIEndpointCapabilityEmbeddings,
		false,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37012), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 1, decision.CandidateCount)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_Enabled_EmbeddingsSkipsChatOnlyStickyBindings(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(10112)
	accounts := []Account{
		{
			ID:          37021,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions"},
			},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
		{
			ID:          37022,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions", "embeddings"},
			},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_embeddings": 37021,
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_embeddings_chat_only", 37021, time.Hour))

	selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
		ctx,
		&groupID,
		"resp_embeddings_chat_only",
		"session_hash_embeddings",
		"text-embedding-3-small",
		nil,
		OpenAIUpstreamTransportHTTPSSE,
		OpenAISchedulerEndpointEmbeddings,
		OpenAIEndpointCapabilityEmbeddings,
		false,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37022), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickyPreviousHit)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, int64(37022), cache.sessionBindings["openai:session_hash_embeddings"])
}

func TestOpenAIGatewayService_OpenAIAccountSchedulerMetrics_DisabledNoOp(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	svc := &OpenAIGatewayService{}
	ttft := 120
	svc.ReportOpenAIAccountScheduleResult(10, true, &ttft)
	svc.RecordOpenAIAccountSwitch()

	snapshot := svc.SnapshotOpenAIAccountSchedulerMetrics()
	require.Equal(t, OpenAIAccountSchedulerMetricsSnapshot{}, snapshot)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyRateLimitedAccountFallsBackToFreshCandidate(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10101)
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	staleSticky := &Account{ID: 31001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0}
	staleBackup := &Account{ID: 31002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	freshSticky := &Account{ID: 31001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, RateLimitResetAt: &rateLimitedUntil}
	freshBackup := &Account{ID: 31002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_rate_limited": 31001}}
	snapshotCache := &openAISnapshotCacheStub{snapshotAccounts: []*Account{staleSticky, staleBackup}, accountsByID: map[int64]*Account{31001: freshSticky, 31002: freshBackup}}
	snapshotService := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{*freshSticky, *freshBackup}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		schedulerSnapshot:  snapshotService,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_rate_limited", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(31002), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyAccountStaysBoundWithoutHealthTier(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10105)
	sticky := Account{ID: 21601, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
	other := Account{ID: 21602, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_sticky": sticky.ID}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{sticky, other}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
		openaiAccountStats: newOpenAIAccountRuntimeStats(),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, sticky.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SkipsAccountAboveGroupUpstreamPriceMaxMultiplier(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10107)
	group := &Group{ID: groupID, UpstreamPriceMaxMultiplier: 0.001}
	channelPrice := 0.002
	account := Account{
		ID:            21711,
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		Priority:      0,
		ChannelPrice:  &channelPrice,
		AccountGroups: []AccountGroup{{GroupID: groupID, Group: group}},
		Groups:        []*Group{group},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{account}}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)

	require.ErrorContains(t, err, "no available OpenAI accounts")
	require.Nil(t, selection)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyAboveGroupUpstreamPriceMaxMultiplierFallsBack(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10108)
	group := &Group{ID: groupID, UpstreamPriceMaxMultiplier: 0.001}
	blockedPrice := 0.002
	allowedPrice := 0.0005
	sticky := Account{
		ID:            21721,
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		Priority:      0,
		ChannelPrice:  &blockedPrice,
		AccountGroups: []AccountGroup{{GroupID: groupID, Group: group}},
		Groups:        []*Group{group},
	}
	available := Account{
		ID:            21722,
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		Priority:      1,
		ChannelPrice:  &allowedPrice,
		AccountGroups: []AccountGroup{{GroupID: groupID, Group: group}},
		Groups:        []*Group{group},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_price_guard": sticky.ID}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{sticky, available}}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_price_guard", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, available.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, 1, cache.deletedSessions["openai:session_hash_price_guard"])
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_LoadBalanceUsesBuiltInCandidateOrder(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10106)
	busy := Account{ID: 21701, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID}}
	available := Account{ID: 21702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{busy, available}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              &config.Config{},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireResults: map[int64]bool{
				busy.ID:      false,
				available.ID: true,
			},
			loadMap: map[int64]*AccountLoadInfo{
				busy.ID:      {AccountID: busy.ID, LoadRate: 100, WaitingCount: 9},
				available.ID: {AccountID: available.ID, LoadRate: 0, WaitingCount: 0},
			},
		}),
		openaiAccountStats: newOpenAIAccountRuntimeStats(),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, available.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_AutoPauseBy5hThreshold(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   95.0,
			"auto_pause_5h_threshold": 0.95,
		},
	}
	secondary := Account{ID: 35002, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35002), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_AllowsBelow5hThreshold(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35101,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   80.0,
			"auto_pause_5h_threshold": 0.95,
		},
	}
	secondary := Account{ID: 35102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35101), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_AutoPauseBy7dThreshold(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35201,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_7d_used_percent":   95.0,
			"auto_pause_7d_threshold": 0.95,
		},
	}
	secondary := Account{ID: 35202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35202), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_UnconfiguredThresholdKeepsLegacyBehavior(t *testing.T) {
	ctx := context.Background()
	primary := Account{ID: 35301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, Extra: map[string]any{"codex_5h_used_percent": 99.0, "codex_7d_used_percent": 99.0}}
	secondary := Account{ID: 35302, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35301), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_UsesGlobalDefaultThreshold(t *testing.T) {
	ctx := withOpenAIQuotaAutoPauseSettings(context.Background(), OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold5h: 0.95})
	primary := Account{
		ID:          35401,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent": 95.0,
		},
	}
	secondary := Account{ID: 35402, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35402), account.ID)
}

// Regression: a per-account explicit-disable flag exempts the account from auto-pause
// even when a global default threshold is set. Without this, "leave threshold blank"
// silently falls back to global default and admins have no way to whitelist a single
// account.
func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_PerAccountDisableOverridesGlobalDefault(t *testing.T) {
	ctx := withOpenAIQuotaAutoPauseSettings(context.Background(), OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold5h: 0.95})
	// Account has high usage AND no per-account threshold (would normally fall back to
	// the global default and get paused), but the explicit disable flag is set.
	primary := Account{
		ID:          35701,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":  99.0,
			"auto_pause_5h_disabled": true,
		},
	}
	secondary := Account{ID: 35702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35701), account.ID)
}

// Disable is per-window: disabling only 5h must still allow 7d auto-pause to fire.
func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_PerWindowDisableScoped(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35801,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   99.0,
			"codex_7d_used_percent":   99.0,
			"auto_pause_5h_disabled":  true,
			"auto_pause_7d_threshold": 0.95,
		},
	}
	secondary := Account{ID: 35802, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35802), account.ID, "7d auto-pause must still fire even though 5h is disabled")
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_StaleUsageWindowResetSkipsPause(t *testing.T) {
	ctx := context.Background()
	// Usage is over threshold but the window's reset time has already passed, so the
	// cached percentage is stale (the real window rolled over) and the account must NOT
	// stay paused — otherwise it could be skipped forever with no traffic to refresh it.
	primary := Account{
		ID:          35501,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   99.0,
			"auto_pause_5h_threshold": 0.95,
			"codex_5h_reset_at":       time.Now().Add(-time.Minute).Format(time.RFC3339),
		},
	}
	secondary := Account{ID: 35502, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35501), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_FreshUsageWindowStillPauses(t *testing.T) {
	ctx := context.Background()
	// Same as above but the window has not reset yet, so the account stays paused.
	primary := Account{
		ID:          35601,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   99.0,
			"auto_pause_5h_threshold": 0.95,
			"codex_5h_reset_at":       time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}
	secondary := Account{ID: 35602, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35602), account.ID)
}

// Issue #2994: an account poisoned with an inflated used% (e.g. from the reverted #2918
// inversion) gets excluded from scheduling, and a paused account never receives traffic to
// refresh its snapshot. When the snapshot is stale (codex_usage_updated_at older than the
// staleness bound) the account must be allowed a request so it can self-heal from the real
// response headers — independent of the window's reset time.
func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_StaleUsageSnapshotSkipsPause_Issue2994(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35701,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   99.0,
			"auto_pause_5h_threshold": 0.95,
			// Window has NOT reset yet, so the reset guard stays inactive.
			"codex_5h_reset_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			// Snapshot is stale: older than openAICodexAutoPauseStaleAfter (2h).
			"codex_usage_updated_at": time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
		},
	}
	secondary := Account{ID: 35702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35701), account.ID)
}

// Issue #2994 guardrail: a genuinely-exhausted account whose snapshot was refreshed recently
// (codex_usage_updated_at fresh) must STILL be auto-paused. The stale self-heal must not let a
// real 99%-used account escape pause.
func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_FreshExhaustedSnapshotStillPauses_Issue2994(t *testing.T) {
	ctx := context.Background()
	primary := Account{
		ID:          35801,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent":   99.0,
			"auto_pause_5h_threshold": 0.95,
			"codex_5h_reset_at":       time.Now().Add(time.Hour).Format(time.RFC3339),
			// Snapshot refreshed 1 minute ago: not stale, so the account stays paused.
			"codex_usage_updated_at": time.Now().Add(-time.Minute).Format(time.RFC3339),
		},
	}
	secondary := Account{ID: 35802, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}, cfg: &config.Config{}}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(35802), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_SkipsFreshlyRateLimitedSnapshotCandidate(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10102)
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	stalePrimary := &Account{ID: 32001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0}
	staleSecondary := &Account{ID: 32002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	freshPrimary := &Account{ID: 32001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, RateLimitResetAt: &rateLimitedUntil}
	freshSecondary := &Account{ID: 32002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	snapshotCache := &openAISnapshotCacheStub{snapshotAccounts: []*Account{stalePrimary, staleSecondary}, accountsByID: map[int64]*Account{32001: freshPrimary, 32002: freshSecondary}}
	snapshotService := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		accountRepo:       schedulerTestOpenAIAccountRepo{accounts: []Account{*freshPrimary, *freshSecondary}},
		cfg:               &config.Config{},
		rateLimitService:  newOpenAIAdvancedSchedulerRateLimitService("true"),
		schedulerSnapshot: snapshotService,
	}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(32002), account.ID)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_ModelRateLimitOnlySkipsThatModel(t *testing.T) {
	ctx := context.Background()
	resetAt := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
	primary := Account{
		ID:          32101,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			modelRateLimitsKey: map[string]any{
				"gpt-5.4": map[string]any{
					"rate_limit_reset_at": resetAt,
				},
			},
		},
	}
	secondary := Account{
		ID:          32102,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    5,
	}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}},
		cfg:         &config.Config{},
	}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.4", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(32102), account.ID)

	account, err = svc.SelectAccountForModelWithExclusions(ctx, nil, "", "gpt-5.3", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(32101), account.ID)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyDBRuntimeRecheckSkipsStaleCachedAccount(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10103)
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	staleSticky := &Account{ID: 33001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0}
	staleBackup := &Account{ID: 33002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	dbSticky := Account{ID: 33001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, RateLimitResetAt: &rateLimitedUntil}
	dbBackup := Account{ID: 33002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_db_runtime_recheck": 33001}}
	snapshotCache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{staleSticky, staleBackup},
		accountsByID:     map[int64]*Account{33001: staleSticky, 33002: staleBackup},
	}
	snapshotService := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{dbSticky, dbBackup}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		schedulerSnapshot:  snapshotService,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_db_runtime_recheck", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(33002), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}

func TestOpenAIGatewayService_SelectAccountForModelWithExclusions_DBRuntimeRecheckSkipsStaleCachedCandidate(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10104)
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	stalePrimary := &Account{ID: 34001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0}
	staleSecondary := &Account{ID: 34002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	dbPrimary := Account{ID: 34001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, RateLimitResetAt: &rateLimitedUntil}
	dbSecondary := Account{ID: 34002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5}
	snapshotCache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{stalePrimary, staleSecondary},
		accountsByID:     map[int64]*Account{34001: stalePrimary, 34002: staleSecondary},
	}
	snapshotService := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		accountRepo:       schedulerTestOpenAIAccountRepo{accounts: []Account{dbPrimary, dbSecondary}},
		cfg:               &config.Config{},
		rateLimitService:  newOpenAIAdvancedSchedulerRateLimitService("true"),
		schedulerSnapshot: snapshotService,
	}

	account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(34002), account.ID)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_PreviousResponseSticky(t *testing.T) {
	ctx := context.Background()
	groupID := int64(9)
	account := Account{
		ID:          1001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
	}
	cache := &schedulerTestGatewayCache{}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 1800
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_prev_001", account.ID, time.Hour))

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"resp_prev_001",
		"session_hash_001",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, account.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
	require.True(t, decision.StickyPreviousHit)
	require.Equal(t, account.ID, cache.sessionBindings["openai:session_hash_001"])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionSticky(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10)
	account := Account{
		ID:          2001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_abc": account.ID,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_abc",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, account.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyBusyKeepsSticky(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10100)
	accounts := []Account{
		{
			ID:          21001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
		{
			ID:          21002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    9,
			GroupIDs:    []int64{groupID},
		},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_sticky_busy": 21001,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 2
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = false
	cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
	cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true

	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{
			21001: false, // sticky 账号已满
			21002: true,  // 若回退负载均衡会命中该账号（本测试要求不能切换）
		},
		waitCounts: map[int64]int{
			21001: 999,
		},
		loadMap: map[int64]*AccountLoadInfo{
			21001: {AccountID: 21001, LoadRate: 90, WaitingCount: 9},
			21002: {AccountID: 21002, LoadRate: 1, WaitingCount: 0},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_sticky_busy",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21001), selection.Account.ID, "busy sticky account should remain selected")
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(21001), selection.WaitPlan.AccountID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyEscapeByTTFT(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10101)
	accounts := []Account{
		{
			ID:          21101,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
		{
			ID:          21102,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			GroupIDs:    []int64{groupID},
		},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_sticky_ttft": 21101}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = true
	cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
	cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
	concurrencyCache := schedulerTestConcurrencyCache{acquireResults: map[int64]bool{21102: true}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
		openaiAccountStats: newOpenAIAccountRuntimeStats(),
	}
	fastTTFT := 14999
	svc.openaiAccountStats.report(21101, true, &fastTTFT)
	stableTTFT := 14999
	svc.openaiAccountStats.report(21101, true, &stableTTFT)

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_ttft", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21101), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	slowTTFT := 20000
	for i := 0; i < 3; i++ {
		svc.openaiAccountStats.report(21101, true, &slowTTFT)
	}

	selection, decision, err = svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_ttft", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21102), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, int64(21101), cache.sessionBindings["openai:session_hash_sticky_ttft"])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyEscapeByErrorRate(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10102)
	accounts := []Account{
		{ID: 21201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}},
		{ID: 21202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_sticky_error_rate": 21201}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = true
	cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
	cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{21202: true}}),
		openaiAccountStats: newOpenAIAccountRuntimeStats(),
	}
	scheduler, ok := svc.getOpenAIAccountScheduler(ctx).(*defaultOpenAIAccountScheduler)
	require.True(t, ok)
	require.NotNil(t, scheduler)
	for i := 0; i < 3; i++ {
		svc.openaiAccountStats.report(21201, false, nil)
	}
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_error_rate", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21201), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
	for i := 0; i < 2; i++ {
		svc.openaiAccountStats.report(21201, false, nil)
	}

	selection, decision, err = svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_error_rate", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21202), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, int64(21201), cache.sessionBindings["openai:session_hash_sticky_error_rate"])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyBusyEscapes(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10103)
	accounts := []Account{
		{ID: 21301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}},
		{ID: 21302, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_sticky_busy_escape": 21301}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = true
	cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
	cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 2
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{21301: false, 21302: true},
		waitCounts:     map[int64]int{21301: 999},
		loadMap: map[int64]*AccountLoadInfo{
			21301: {AccountID: 21301, LoadRate: 95, WaitingCount: 9},
			21302: {AccountID: 21302, LoadRate: 1, WaitingCount: 0},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_busy_escape", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21302), selection.Account.ID)
	require.Nil(t, selection.WaitPlan)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionStickyEscapeDisabledReturnsStickyWaitPlan(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10104)
	accounts := []Account{
		{ID: 21401, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}},
		{ID: 21402, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session_hash_sticky_disabled": 21401}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = false
	cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
	cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 2
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{21401: false, 21402: true},
		waitCounts:     map[int64]int{21401: 999},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
		openaiAccountStats: newOpenAIAccountRuntimeStats(),
	}
	slowTTFT := 20000
	svc.openaiAccountStats.report(21401, true, &slowTTFT)
	for i := 0; i < 5; i++ {
		svc.openaiAccountStats.report(21401, false, nil)
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_sticky_disabled", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21401), selection.Account.ID)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(21401), selection.WaitPlan.AccountID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SubscriptionPriorityChoosesSubscriptionPoolFirst(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10120)
	accounts := []Account{
		{
			ID:          21601,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    10,
			GroupIDs:    []int64{groupID},
			Credentials: map[string]any{"plan_type": "plus"},
		},
		{
			ID:          21602,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
	}
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{21601: true, 21602: true},
		loadMap: map[int64]*AccountLoadInfo{
			21601: {AccountID: 21601, LoadRate: 90, WaitingCount: 1},
			21602: {AccountID: 21602, LoadRate: 0, WaitingCount: 0},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "", "true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_subscription_first", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21601), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 1, decision.TopK)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SubscriptionPriorityFallsBackWhenSubscriptionFull(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10121)
	accounts := []Account{
		{
			ID:          21611,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Credentials: map[string]any{"plan_type": "team"},
		},
		{
			ID:          21612,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    9,
			GroupIDs:    []int64{groupID},
		},
	}
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{21611: false, 21612: true},
		loadMap: map[int64]*AccountLoadInfo{
			21611: {AccountID: 21611, LoadRate: 0, WaitingCount: 0},
			21612: {AccountID: 21612, LoadRate: 90, WaitingCount: 1},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "", "true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_subscription_fallback", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21612), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.True(t, selection.Acquired)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SubscriptionPriorityDisabledUsesScore(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10122)
	accounts := []Account{
		{
			ID:          21621,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    10,
			GroupIDs:    []int64{groupID},
			Credentials: map[string]any{"plan_type": "pro"},
		},
		{
			ID:          21622,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
	}
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{21621: true, 21622: true},
		loadMap: map[int64]*AccountLoadInfo{
			21621: {AccountID: 21621, LoadRate: 90, WaitingCount: 1},
			21622: {AccountID: 21622, LoadRate: 0, WaitingCount: 0},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "", "false"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_subscription_disabled", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21622), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_UsesAccountGroupPriorityWithinGroupPool(t *testing.T) {
	ctx := context.Background()
	groupID := int64(10123)
	accounts := []Account{
		{
			ID:          21631,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			AccountGroups: []AccountGroup{
				{AccountID: 21631, GroupID: groupID, Priority: 100},
			},
			GroupIDs: []int64{groupID},
		},
		{
			ID:          21632,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    100000,
			AccountGroups: []AccountGroup{
				{AccountID: 21632, GroupID: groupID, Priority: 1},
			},
			GroupIDs: []int64{groupID},
		},
	}
	cfg := newSchedulerTestSubscriptionPriorityConfig()
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 0
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_group_priority", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(21632), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIAccountSchedulingPriorityUsesCurrentGroupMembership(t *testing.T) {
	groupID := int64(10124)
	otherGroupID := int64(10125)
	account := &Account{
		Priority: 99,
		AccountGroups: []AccountGroup{
			{GroupID: otherGroupID, Priority: 50},
			{GroupID: groupID, Priority: 2},
		},
	}

	require.Equal(t, 2, openAIAccountSchedulingPriority(account, &groupID))
	require.Equal(t, 99, openAIAccountSchedulingPriority(account, nil))
	require.Equal(t, 99, openAIAccountSchedulingPriority(account, ptrInt64(99999)))
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_BalancedPreservesHardFiltersAndSlotFailover(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	ctx := context.Background()
	groupID := int64(10126)
	excluded := Account{ID: 21640, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": true}}
	compactUnsupported := Account{ID: 21641, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": false}}
	fastBusy := Account{ID: 21642, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": true}}
	fallback := Account{ID: 21643, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": true}}
	now := time.Now()
	healthKeyService := &OpenAIGatewayService{}
	fastKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&fastBusy, OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: openAISchedulerHealthEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny, RequireCompact: true})
	fallbackKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&fallback, OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: openAISchedulerHealthEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny, RequireCompact: true})
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		fastKey:     {Key: fastKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(time.Hour)},
		fallbackKey: {Key: fallbackKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 500, ExpiresAt: now.Add(time.Hour)},
	}}
	acquireOrder := []int64{}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{excluded, compactUnsupported, fastBusy, fallback}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireOrder: &acquireOrder,
			acquireResults: map[int64]bool{
				fastBusy.ID: false,
				fallback.ID: true,
			},
		}),
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "balanced_slot_failover", "gpt-5.4", map[int64]struct{}{excluded.ID: {}}, OpenAIUpstreamTransportAny, true)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, fallback.ID, selection.Account.ID)
	require.Equal(t, []int64{fastBusy.ID, fallback.ID}, acquireOrder)
	require.Equal(t, 1, healthRepo.getCalls)
	require.ElementsMatch(t, []OpenAISchedulerHealthKey{fastKey, fallbackKey}, healthRepo.keys)
	require.Equal(t, 2, decision.CandidateCount)
	require.Equal(t, 2, decision.TopK)
}

func TestDefaultOpenAIAccountScheduler_BalancedLatencyTailUsesLegacySelectionOrderPosition(t *testing.T) {
	accounts := []*Account{
		{ID: 21644, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 21645, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 21646, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	svc := &OpenAIGatewayService{}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(nil))
	scheduler := &defaultOpenAIAccountScheduler{service: svc}
	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportHTTPSSE, balancedHealthAttempted: true,
		balancedHealthSnapshots: make(map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot),
	}
	now := time.Now()
	for i, account := range accounts {
		key := svc.openAIBalancedHealthKeyForCandidate(account, req)
		ttft := []float64{100, 4000, 3000}[i]
		req.balancedHealthSnapshots[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: ttft, ExpiresAt: now.Add(time.Hour),
		}
	}
	loadInfo := func(accountID int64) *AccountLoadInfo { return &AccountLoadInfo{AccountID: accountID} }
	plan := openAIAccountLoadPlan{
		allCandidates: []openAIAccountCandidateScore{
			{account: accounts[0], loadInfo: loadInfo(accounts[0].ID)},
			{account: accounts[1], loadInfo: loadInfo(accounts[1].ID)},
			{account: accounts[2], loadInfo: loadInfo(accounts[2].ID)},
		},
		candidates: []openAIAccountCandidateScore{
			{account: accounts[0], loadInfo: loadInfo(accounts[0].ID)},
			{account: accounts[1], loadInfo: loadInfo(accounts[1].ID)},
			{account: accounts[2], loadInfo: loadInfo(accounts[2].ID)},
		},
		selectionOrder: []openAIAccountCandidateScore{
			{account: accounts[2], loadInfo: loadInfo(accounts[2].ID)},
			{account: accounts[1], loadInfo: loadInfo(accounts[1].ID)},
			{account: accounts[0], loadInfo: loadInfo(accounts[0].ID)},
		},
	}

	scheduler.applyOpenAIBalancedSelectionOrder(context.Background(), req, &plan)
	orderedIDs := make([]int64, 0, len(plan.selectionOrder))
	for _, candidate := range plan.selectionOrder {
		orderedIDs = append(orderedIDs, candidate.account.ID)
	}
	require.Equal(t, []int64{accounts[0].ID, accounts[2].ID, accounts[1].ID}, orderedIDs)
}

func TestDefaultOpenAIAccountScheduler_RuntimeSettingsMapBalancedControls(t *testing.T) {
	repo := &openAIAutoSchedulerSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIAutoSchedulerSettings: `{"mode":"balanced","shadow_mode":true,"top_k":7,"exploration_rate":0.08,"session_escape_min_gap_ms":2500,"session_escape_ratio":0.5,"probe_interval_seconds":60}`,
	}}
	gateway := &OpenAIGatewayService{settingService: NewSettingService(repo, &config.Config{})}
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}

	req := scheduler.withOpenAIBalancedRuntimeSettings(context.Background(), OpenAIAccountScheduleRequest{})

	require.Equal(t, OpenAIAutoSchedulerModeBalanced, req.balancedMode)
	require.True(t, req.balancedShadowMode)
	require.Equal(t, 7, req.balancedPolicySettings.TopK)
	require.InDelta(t, 0.08, req.balancedPolicySettings.ExplorationRate, 0.0001)
	require.InDelta(t, 2500, req.balancedPolicySettings.SessionEscapeMinGapMS, 0.0001)
	require.InDelta(t, 0.5, req.balancedPolicySettings.SessionEscapeRatio, 0.0001)

	_ = scheduler.withOpenAIBalancedRuntimeSettings(context.Background(), OpenAIAccountScheduleRequest{})
	require.Equal(t, 1, repo.getValueCalls)
}

func TestDefaultOpenAIAccountScheduler_ShadowKeepsLegacyPlanAndLiveUsesBalancedPlan(t *testing.T) {
	accounts := []*Account{
		{ID: 216440, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 216441, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	loadInfo := func(accountID int64) *AccountLoadInfo { return &AccountLoadInfo{AccountID: accountID} }
	newPlan := func() openAIAccountLoadPlan {
		first := openAIAccountCandidateScore{account: accounts[0], loadInfo: loadInfo(accounts[0].ID)}
		second := openAIAccountCandidateScore{account: accounts[1], loadInfo: loadInfo(accounts[1].ID)}
		return openAIAccountLoadPlan{
			allCandidates:  []openAIAccountCandidateScore{first, second},
			candidates:     []openAIAccountCandidateScore{first, second},
			selectionOrder: []openAIAccountCandidateScore{first, second},
		}
	}
	gateway := &OpenAIGatewayService{}
	gateway.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(nil))
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}
	baseReq := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportHTTPSSE, balancedMode: OpenAIAutoSchedulerModeBalanced,
		balancedHealthAttempted: true, balancedHealthSnapshots: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{},
	}
	now := time.Now()
	for i, account := range accounts {
		key := gateway.openAIBalancedHealthKeyForCandidate(account, baseReq)
		baseReq.balancedHealthSnapshots[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning,
			PredictedTTFTMS: []float64{2000, 500}[i], ExpiresAt: now.Add(time.Hour),
		}
	}

	shadowReq := baseReq
	shadowReq.balancedShadowMode = true
	shadowReq.balancedPolicySettings = OpenAIBalancedSettings{Mode: OpenAIAutoSchedulerModeBalanced, ShadowMode: true, TopK: 2}
	shadowPlan := newPlan()
	scheduler.applyOpenAIBalancedSelectionOrder(context.Background(), shadowReq, &shadowPlan)
	require.Equal(t, []int64{accounts[0].ID, accounts[1].ID}, openAIAccountCandidateIDs(shadowPlan.selectionOrder))

	liveReq := baseReq
	liveReq.balancedPolicySettings = OpenAIBalancedSettings{Mode: OpenAIAutoSchedulerModeBalanced, TopK: 2}
	livePlan := newPlan()
	scheduler.applyOpenAIBalancedSelectionOrder(context.Background(), liveReq, &livePlan)
	require.Equal(t, []int64{accounts[1].ID, accounts[0].ID}, openAIAccountCandidateIDs(livePlan.selectionOrder))

	legacyReq := baseReq
	legacyReq.balancedMode = OpenAIAutoSchedulerModeLegacy
	legacyPlan := newPlan()
	scheduler.applyOpenAIBalancedSelectionOrder(context.Background(), legacyReq, &legacyPlan)
	require.Equal(t, []int64{accounts[0].ID, accounts[1].ID}, openAIAccountCandidateIDs(legacyPlan.selectionOrder))
}

func TestDefaultOpenAIAccountScheduler_SoftStickyUsesRuntimeEscapeThresholds(t *testing.T) {
	accounts := []*Account{
		{ID: 216442, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 216443, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	gateway := &OpenAIGatewayService{}
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}
	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportHTTPSSE, balancedMode: OpenAIAutoSchedulerModeBalanced,
		balancedAccounts: accounts, balancedLoadMap: map[int64]*AccountLoadInfo{}, balancedHealthAttempted: true,
		balancedHealthSnapshots: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{},
		balancedPolicySettings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, TopK: 2,
			SessionEscapeMinGapMS: 0, SessionEscapeRatio: 0,
		},
	}
	now := time.Now()
	for i, account := range accounts {
		key := gateway.openAIBalancedHealthKeyForCandidate(account, req)
		req.balancedHealthSnapshots[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning,
			PredictedTTFTMS: []float64{900, 500}[i], ExpiresAt: now.Add(time.Hour),
		}
	}

	require.Equal(t, "ttft", scheduler.openAIBalancedSoftStickyEscapeReason(req, accounts[0].ID))
}

func TestDefaultOpenAIAccountScheduler_ComputesStickyShadowDecisionWithoutChangingLegacyAccount(t *testing.T) {
	accounts := []*Account{
		{ID: 216444, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 216445, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	gateway := &OpenAIGatewayService{}
	gateway.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(nil))
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}
	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportHTTPSSE, balancedMode: OpenAIAutoSchedulerModeBalanced,
		balancedShadowMode: true, balancedAccounts: accounts, balancedLoadMap: map[int64]*AccountLoadInfo{},
		balancedHealthAttempted: true, balancedHealthSnapshots: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{},
		balancedPolicySettings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, ShadowMode: true, TopK: 2,
			SessionEscapeMinGapMS: 100, SessionEscapeRatio: 0.1,
		},
	}
	now := time.Now()
	for i, account := range accounts {
		key := gateway.openAIBalancedHealthKeyForCandidate(account, req)
		req.balancedHealthSnapshots[key] = OpenAISchedulerHealthSnapshot{
			Key: key, State: OpenAIAutoSchedulerStateRunning,
			PredictedTTFTMS: []float64{2000, 500}[i], ExpiresAt: now.Add(time.Hour),
		}
	}

	result, ok := scheduler.openAIBalancedShadowDecisionForSticky(context.Background(), req, accounts[0].ID)

	require.True(t, ok)
	require.True(t, result.Shadow)
	require.Equal(t, accounts[0].ID, result.LegacyAccountID)
	require.Equal(t, accounts[1].ID, result.ShadowAccountID)
	require.Equal(t, []int64{accounts[0].ID, accounts[1].ID}, result.OrderedAccountIDs)
}

func openAIAccountCandidateIDs(candidates []openAIAccountCandidateScore) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.account != nil {
			ids = append(ids, candidate.account.ID)
		}
	}
	return ids
}

func TestOpenAIGatewayService_SelectAccountBalancedCompactStaleRetryHonorsCircuit(t *testing.T) {
	groupID := int64(101261)
	busy := &Account{
		ID: 21647, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": true},
	}
	staleUnsupported := &Account{
		ID: 21648, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}, Extra: map[string]any{"openai_compact_supported": false},
	}
	freshSupported := *staleUnsupported
	freshSupported.Extra = map[string]any{"openai_compact_supported": true}
	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.4", RequiredEndpoint: OpenAISchedulerEndpointResponses,
		RequiredTransport: OpenAIUpstreamTransportAny, RequireCompact: true,
	}
	baseReq := req
	keyService := &OpenAIGatewayService{}
	busyKey := keyService.openAIBalancedHealthKeyForCandidate(busy, req)
	staleKey := keyService.openAIBalancedHealthKeyForCandidate(staleUnsupported, req)
	now := time.Now()
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		busyKey:  {Key: busyKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(time.Hour)},
		staleKey: {Key: staleKey, State: OpenAIAutoSchedulerStateOpen, PredictedTTFTMS: 100, ExpiresAt: now.Add(time.Hour)},
	}}
	acquireOrder := []int64{}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{*busy, freshSupported}}},
		cfg:         newSchedulerTestSubscriptionPriorityConfig(),
		schedulerSnapshot: &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{
			accountsByID: map[int64]*Account{busy.ID: busy, staleUnsupported.ID: staleUnsupported},
		}},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireOrder: &acquireOrder, acquireResults: map[int64]bool{busy.ID: false, staleUnsupported.ID: true}}),
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))
	scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
	req = scheduler.preloadOpenAIBalancedHealth(context.Background(), req, []*Account{busy, staleUnsupported}, nil)
	busyCandidate := openAIAccountCandidateScore{account: busy, loadInfo: &AccountLoadInfo{AccountID: busy.ID}}
	staleCandidate := openAIAccountCandidateScore{account: staleUnsupported, loadInfo: &AccountLoadInfo{AccountID: staleUnsupported.ID}}
	plan := openAIAccountLoadPlan{
		allCandidates:             []openAIAccountCandidateScore{busyCandidate, staleCandidate},
		candidates:                []openAIAccountCandidateScore{busyCandidate},
		staleSnapshotCompactRetry: []openAIAccountCandidateScore{staleCandidate},
		topK:                      2,
	}
	plan.selectionOrder = scheduler.buildOpenAISelectionOrder(req, plan)
	selection, compactBlocked, err := scheduler.tryAcquireOpenAISelectionOrder(context.Background(), req, plan.selectionOrder)
	require.NoError(t, err)
	require.Nil(t, selection)
	require.False(t, compactBlocked)
	require.NotContains(t, acquireOrder, staleUnsupported.ID)
	require.Equal(t, 1, healthRepo.getCalls)
	require.ElementsMatch(t, []OpenAISchedulerHealthKey{busyKey, staleKey}, healthRepo.keys)

	waitSelection, _, _, _, err := scheduler.finishLoadBalanceSelectionFallback(context.Background(), req, openAIAccountLoadSelectionAttempt{
		selectionOrder: plan.selectionOrder,
	})
	require.NoError(t, err)
	require.NotNil(t, waitSelection)
	require.NotNil(t, waitSelection.WaitPlan)
	require.Equal(t, busy.ID, waitSelection.WaitPlan.AccountID)

	healthRepo.err = errors.New("health unavailable")
	healthRepo.getCalls = 0
	healthRepo.keys = nil
	acquireOrder = nil
	legacyReq := scheduler.preloadOpenAIBalancedHealth(context.Background(), baseReq, []*Account{busy, staleUnsupported}, nil)
	legacyPlan := openAIAccountLoadPlan{
		allCandidates:             []openAIAccountCandidateScore{busyCandidate, staleCandidate},
		candidates:                []openAIAccountCandidateScore{busyCandidate},
		staleSnapshotCompactRetry: []openAIAccountCandidateScore{staleCandidate},
		topK:                      2,
	}
	legacyPlan.selectionOrder = scheduler.buildOpenAISelectionOrder(legacyReq, legacyPlan)
	legacySelection, _, err := scheduler.tryAcquireOpenAISelectionOrder(context.Background(), legacyReq, legacyPlan.selectionOrder)
	require.NoError(t, err)
	require.NotNil(t, legacySelection)
	require.True(t, legacySelection.Acquired)
	require.Equal(t, staleUnsupported.ID, legacySelection.Account.ID)
	require.Equal(t, []int64{busy.ID, staleUnsupported.ID}, acquireOrder)
	require.Equal(t, 1, healthRepo.getCalls)
	require.ElementsMatch(t, []OpenAISchedulerHealthKey{busyKey, staleKey}, healthRepo.keys)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_BalancedHealthFallbackUsesLegacyOrder(t *testing.T) {
	groupID := int64(10127)
	legacyFirst := Account{ID: 21650, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
	healthFirst := Account{ID: 21651, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID}}
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: openAISchedulerHealthEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
	healthKeyService := &OpenAIGatewayService{}
	legacyKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&legacyFirst, req)
	healthKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&healthFirst, req)
	now := time.Now()
	freshStates := map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		legacyKey: {Key: legacyKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 2000, ExpiresAt: now.Add(time.Hour)},
		healthKey: {Key: healthKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(time.Hour)},
	}
	tests := []struct {
		name string
		repo *balancedSchedulerHealthRepoStub
	}{
		{name: "repository error", repo: &balancedSchedulerHealthRepoStub{err: errors.New("health unavailable")}},
		{name: "missing snapshot", repo: &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{legacyKey: freshStates[legacyKey]}}},
		{name: "expired snapshot", repo: &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
			legacyKey: freshStates[legacyKey],
			healthKey: {Key: healthKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(-time.Second)},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{healthFirst, legacyFirst}},
				cache:              &schedulerTestGatewayCache{},
				cfg:                newSchedulerTestSubscriptionPriorityConfig(),
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(tt.repo))

			selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, legacyFirst.ID, selection.Account.ID)
			require.Equal(t, 1, tt.repo.getCalls)
		})
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_BalancedHealthBatchIsNotRepeatedAfterFreshLoad(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(10128)
	first := Account{ID: 21660, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}}
	second := Account{ID: 21661, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID}}
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: openAISchedulerHealthEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
	healthKeyService := &OpenAIGatewayService{}
	firstKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&first, req)
	secondKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&second, req)
	now := time.Now()
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		firstKey:  {Key: firstKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(time.Hour)},
		secondKey: {Key: secondKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 300, ExpiresAt: now.Add(time.Hour)},
	}}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{first, second}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			first.ID: false, second.ID: false,
		}}),
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "balanced_health_single_batch", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, 1, healthRepo.getCalls)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_BalancedHealthBatchIsSharedAcrossSubscriptionPools(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(10129)
	subscription := Account{ID: 21670, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "plus"}}
	regular := Account{ID: 21671, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}}
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.4", RequiredEndpoint: openAISchedulerHealthEndpointResponses, RequiredTransport: OpenAIUpstreamTransportAny}
	healthKeyService := &OpenAIGatewayService{}
	subscriptionKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&subscription, req)
	regularKey := healthKeyService.openAIBalancedHealthKeyForCandidate(&regular, req)
	now := time.Now()
	healthRepo := &balancedSchedulerHealthRepoStub{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		subscriptionKey: {Key: subscriptionKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 200, ExpiresAt: now.Add(time.Hour)},
		regularKey:      {Key: regularKey, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 300, ExpiresAt: now.Add(time.Hour)},
	}}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{subscription, regular}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true", "true", "true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{
			subscription.ID: false, regular.ID: true,
		}}),
	}
	svc.SetOpenAIBalancedScheduler(NewOpenAIBalancedScheduler(healthRepo))

	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "balanced_subscription_batch", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, regular.ID, selection.Account.ID)
	require.Equal(t, 1, healthRepo.getCalls)
	require.ElementsMatch(t, []OpenAISchedulerHealthKey{subscriptionKey, regularKey}, healthRepo.keys)
}

func TestDefaultOpenAIAccountScheduler_ShouldEscapeStickyAccount_ThresholdBoundary(t *testing.T) {
	stats := newOpenAIAccountRuntimeStats()
	accountID := int64(21501)
	ttft := 15000
	stats.report(accountID, true, &ttft)
	stats.report(accountID, false, nil)
	stats.report(accountID, true, nil)
	scheduler := &defaultOpenAIAccountScheduler{stats: stats}

	reason, errorRate, observedTTFT, shouldEscape := scheduler.shouldEscapeStickyAccount(accountID, openAIStickyEscapeConfig{
		enabled:   true,
		ttftMs:    15000,
		errorRate: 0.5,
	})
	require.False(t, shouldEscape)
	require.Empty(t, reason)
	require.InDelta(t, 0.16, errorRate, 1e-9)
	require.InDelta(t, 15000, observedTTFT, 1e-9)

	for i := 0; i < 4; i++ {
		stats.report(accountID, false, nil)
	}
	reason, errorRate, _, shouldEscape = scheduler.shouldEscapeStickyAccount(accountID, openAIStickyEscapeConfig{
		enabled:   true,
		ttftMs:    15000,
		errorRate: 1,
	})
	require.False(t, shouldEscape)
	require.Empty(t, reason)
	reason, errorRate, observedTTFT, shouldEscape = scheduler.shouldEscapeStickyAccount(accountID, openAIStickyEscapeConfig{
		enabled:   true,
		ttftMs:    15000,
		errorRate: errorRate,
	})
	require.False(t, shouldEscape)
	require.Empty(t, reason)
	require.InDelta(t, 0.655936, errorRate, 1e-9)
	require.InDelta(t, 15000, observedTTFT, 1e-9)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SessionSticky_ForceHTTP(t *testing.T) {
	ctx := context.Background()
	groupID := int64(1010)
	account := Account{
		ID:          2101,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
		Extra: map[string]any{
			"openai_ws_force_http": true,
		},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_force_http": account.ID,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_force_http",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, account.ID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_RequiredWSV2_SkipsStickyHTTPAccount(t *testing.T) {
	ctx := context.Background()
	groupID := int64(1011)
	accounts := []Account{
		{
			ID:          2201,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
		},
		{
			ID:          2202,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			GroupIDs:    []int64{groupID},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_ws_only": 2201,
		},
	}
	cfg := newSchedulerTestOpenAIWSV2Config()

	// 构造“HTTP-only 账号负载更低”的场景，验证 required transport 会强制过滤。
	concurrencyCache := schedulerTestConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			2201: {AccountID: 2201, LoadRate: 0, WaitingCount: 0},
			2202: {AccountID: 2202, LoadRate: 90, WaitingCount: 5},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_ws_only",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportResponsesWebsocketV2,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2202), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, 1, decision.CandidateCount)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_ClearsStickyAccountOutsideGroup(t *testing.T) {
	ctx := context.Background()
	groupID := int64(1013)
	accounts := []Account{
		{
			ID:          2401,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
		{
			ID:          2402,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
			AccountGroups: []AccountGroup{
				{AccountID: 2402, GroupID: groupID},
			},
		},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_removed_group": 2401,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_hash_removed_group",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2402), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.False(t, decision.StickySessionHit)
	require.Equal(t, 1, cache.deletedSessions["openai:session_hash_removed_group"])
	require.Equal(t, int64(2402), cache.sessionBindings["openai:session_hash_removed_group"])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_RequiredWSV2_NoAvailableAccount(t *testing.T) {
	ctx := context.Background()
	groupID := int64(1012)
	accounts := []Account{
		{
			ID:          2301,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestOpenAIWSV2Config(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportResponsesWebsocketV2,
		false,
	)
	require.Error(t, err)
	require.Nil(t, selection)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 0, decision.CandidateCount)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_LoadBalanceTopKFallback(t *testing.T) {
	ctx := context.Background()
	groupID := int64(11)
	accounts := []Account{
		{
			ID:          3001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
		{
			ID:          3002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
		{
			ID:          3003,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
		},
	}

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 0.4
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1.0
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1.0
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.1

	concurrencyCache := schedulerTestConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			3001: {AccountID: 3001, LoadRate: 95, WaitingCount: 8},
			3002: {AccountID: 3002, LoadRate: 20, WaitingCount: 1},
			3003: {AccountID: 3003, LoadRate: 10, WaitingCount: 0},
		},
		acquireResults: map[int64]bool{
			3003: false, // top1 失败，必须回退到 top-K 的下一候选
			3002: true,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(3002), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 3, decision.CandidateCount)
	require.Equal(t, 2, decision.TopK)
	require.Greater(t, decision.LoadSkew, 0.0)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

// Regression: TopK initial filter must drop quota-auto-paused accounts. Otherwise
// the candidate pool is filled with paused accounts, healthy accounts fall outside
// TopK, and the scheduler returns "no available accounts" even though healthy ones
// exist.
func TestOpenAIGatewayService_SelectAccountWithScheduler_LoadBalanceTopKExcludesQuotaPaused(t *testing.T) {
	ctx := context.Background()
	groupID := int64(110)
	accounts := []Account{
		{
			ID:          37001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			Extra: map[string]any{
				"codex_5h_used_percent":   96.0,
				"auto_pause_5h_threshold": 0.95,
			},
		},
		{
			ID:          37002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    5,
		},
	}

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1 // TopK=1 makes the bug fatal: paused account would crowd out the healthy one entirely
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 0.4
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1.0
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1.0

	concurrencyCache := schedulerTestConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			37001: {AccountID: 37001, LoadRate: 5, WaitingCount: 0},
			37002: {AccountID: 37002, LoadRate: 5, WaitingCount: 0},
		},
		acquireResults: map[int64]bool{
			37002: true,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(37002), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	// Only the healthy account should ever enter the candidate pool; the paused one
	// must be filtered out at the initial-filter stage.
	require.Equal(t, 1, decision.CandidateCount)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAIGatewayService_OpenAIAccountSchedulerMetrics(t *testing.T) {
	ctx := context.Background()
	groupID := int64(12)
	account := Account{
		ID:          4001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{
		sessionBindings: map[string]int64{
			"openai:session_hash_metrics": account.ID,
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "session_hash_metrics", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	svc.ReportOpenAIAccountScheduleResult(account.ID, true, intPtrForTest(120))
	svc.RecordOpenAIAccountSwitch()

	snapshot := svc.SnapshotOpenAIAccountSchedulerMetrics()
	require.GreaterOrEqual(t, snapshot.SelectTotal, int64(1))
	require.GreaterOrEqual(t, snapshot.StickySessionHitTotal, int64(1))
	require.GreaterOrEqual(t, snapshot.AccountSwitchTotal, int64(1))
	require.GreaterOrEqual(t, snapshot.SchedulerLatencyMsAvg, float64(0))
	require.GreaterOrEqual(t, snapshot.StickyHitRatio, 0.0)
	require.GreaterOrEqual(t, snapshot.RuntimeStatsAccountCount, 1)
}

func intPtrForTest(v int) *int {
	return &v
}

func TestOpenAIAccountRuntimeStats_ReportAndSnapshot(t *testing.T) {
	stats := newOpenAIAccountRuntimeStats()
	stats.report(1001, true, nil)
	firstTTFT := 100
	stats.report(1001, false, &firstTTFT)
	secondTTFT := 200
	stats.report(1001, false, &secondTTFT)

	errorRate, ttft, hasTTFT := stats.snapshot(1001)
	require.True(t, hasTTFT)
	require.InDelta(t, 0.36, errorRate, 1e-9)
	require.InDelta(t, 120.0, ttft, 1e-9)
	require.Equal(t, 1, stats.size())
}

func TestOpenAIAccountRuntimeStats_ReportConcurrent(t *testing.T) {
	stats := newOpenAIAccountRuntimeStats()

	const (
		accountCount = 4
		workers      = 16
		iterations   = 800
	)
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				accountID := int64(i%accountCount + 1)
				success := (i+worker)%3 != 0
				ttft := 80 + (i+worker)%40
				stats.report(accountID, success, &ttft)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, accountCount, stats.size())
	for accountID := int64(1); accountID <= accountCount; accountID++ {
		errorRate, ttft, hasTTFT := stats.snapshot(accountID)
		require.GreaterOrEqual(t, errorRate, 0.0)
		require.LessOrEqual(t, errorRate, 1.0)
		require.True(t, hasTTFT)
		require.Greater(t, ttft, 0.0)
	}
}

func TestSelectTopKOpenAICandidates(t *testing.T) {
	candidates := []openAIAccountCandidateScore{
		{
			account:  &Account{ID: 11, Priority: 2},
			loadInfo: &AccountLoadInfo{LoadRate: 10, WaitingCount: 1},
			score:    10.0,
			priority: 2,
		},
		{
			account:  &Account{ID: 12, Priority: 1},
			loadInfo: &AccountLoadInfo{LoadRate: 20, WaitingCount: 1},
			score:    9.5,
			priority: 1,
		},
		{
			account:  &Account{ID: 13, Priority: 1},
			loadInfo: &AccountLoadInfo{LoadRate: 30, WaitingCount: 0},
			score:    10.0,
			priority: 1,
		},
		{
			account:  &Account{ID: 14, Priority: 0},
			loadInfo: &AccountLoadInfo{LoadRate: 40, WaitingCount: 0},
			score:    8.0,
			priority: 0,
		},
	}

	top2 := selectTopKOpenAICandidates(candidates, 2)
	require.Len(t, top2, 2)
	require.Equal(t, int64(13), top2[0].account.ID)
	require.Equal(t, int64(11), top2[1].account.ID)

	topAll := selectTopKOpenAICandidates(candidates, 8)
	require.Len(t, topAll, len(candidates))
	require.Equal(t, int64(13), topAll[0].account.ID)
	require.Equal(t, int64(11), topAll[1].account.ID)
	require.Equal(t, int64(12), topAll[2].account.ID)
	require.Equal(t, int64(14), topAll[3].account.ID)
}

func TestBuildOpenAIWeightedSelectionOrder_DeterministicBySessionSeed(t *testing.T) {
	candidates := []openAIAccountCandidateScore{
		{
			account:  &Account{ID: 101},
			loadInfo: &AccountLoadInfo{LoadRate: 10, WaitingCount: 0},
			score:    4.2,
		},
		{
			account:  &Account{ID: 102},
			loadInfo: &AccountLoadInfo{LoadRate: 30, WaitingCount: 1},
			score:    3.5,
		},
		{
			account:  &Account{ID: 103},
			loadInfo: &AccountLoadInfo{LoadRate: 50, WaitingCount: 2},
			score:    2.1,
		},
	}
	req := OpenAIAccountScheduleRequest{
		GroupID:        int64PtrForTest(99),
		SessionHash:    "session_seed_fixed",
		RequestedModel: "gpt-5.1",
	}

	first := buildOpenAIWeightedSelectionOrder(candidates, req)
	second := buildOpenAIWeightedSelectionOrder(candidates, req)
	require.Len(t, first, len(candidates))
	require.Len(t, second, len(candidates))
	for i := range first {
		require.Equal(t, first[i].account.ID, second[i].account.ID)
	}
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_LoadBalanceDistributesAcrossSessions(t *testing.T) {
	ctx := context.Background()
	groupID := int64(15)
	accounts := []Account{
		{
			ID:          5101,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 3,
			Priority:    0,
		},
		{
			ID:          5102,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 3,
			Priority:    0,
		},
		{
			ID:          5103,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 3,
			Priority:    0,
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 3
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 1

	concurrencyCache := schedulerTestConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			5101: {AccountID: 5101, LoadRate: 20, WaitingCount: 1},
			5102: {AccountID: 5102, LoadRate: 20, WaitingCount: 1},
			5103: {AccountID: 5103, LoadRate: 20, WaitingCount: 1},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{sessionBindings: map[string]int64{}},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selected := make(map[int64]int, len(accounts))
	for i := 0; i < 60; i++ {
		sessionHash := fmt.Sprintf("session_hash_lb_%d", i)
		selection, decision, err := svc.SelectAccountWithScheduler(
			ctx,
			&groupID,
			"",
			sessionHash,
			"gpt-5.1",
			nil,
			OpenAIUpstreamTransportAny,
			false,
		)
		require.NoError(t, err)
		require.NotNil(t, selection)
		require.NotNil(t, selection.Account)
		require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
		selected[selection.Account.ID]++
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	}

	// 多 session 应该能打散到多个账号，避免“恒定单账号命中”。
	require.GreaterOrEqual(t, len(selected), 2)
}

func TestDeriveOpenAISelectionSeed_NoAffinityAddsEntropy(t *testing.T) {
	req := OpenAIAccountScheduleRequest{
		RequestedModel: "gpt-5.1",
	}
	seed1 := deriveOpenAISelectionSeed(req)
	time.Sleep(1 * time.Millisecond)
	seed2 := deriveOpenAISelectionSeed(req)
	require.NotZero(t, seed1)
	require.NotZero(t, seed2)
	require.NotEqual(t, seed1, seed2)
}

func TestBuildOpenAIWeightedSelectionOrder_HandlesInvalidScores(t *testing.T) {
	candidates := []openAIAccountCandidateScore{
		{
			account:  &Account{ID: 901},
			loadInfo: &AccountLoadInfo{LoadRate: 5, WaitingCount: 0},
			score:    math.NaN(),
		},
		{
			account:  &Account{ID: 902},
			loadInfo: &AccountLoadInfo{LoadRate: 5, WaitingCount: 0},
			score:    math.Inf(1),
		},
		{
			account:  &Account{ID: 903},
			loadInfo: &AccountLoadInfo{LoadRate: 5, WaitingCount: 0},
			score:    -1,
		},
	}
	req := OpenAIAccountScheduleRequest{
		SessionHash: "seed_invalid_scores",
	}

	order := buildOpenAIWeightedSelectionOrder(candidates, req)
	require.Len(t, order, len(candidates))
	seen := map[int64]struct{}{}
	for _, item := range order {
		seen[item.account.ID] = struct{}{}
	}
	require.Len(t, seen, len(candidates))
}

func TestOpenAISelectionRNG_SeedZeroStillWorks(t *testing.T) {
	rng := newOpenAISelectionRNG(0)
	v1 := rng.nextUint64()
	v2 := rng.nextUint64()
	require.NotEqual(t, v1, v2)
	require.GreaterOrEqual(t, rng.nextFloat64(), 0.0)
	require.Less(t, rng.nextFloat64(), 1.0)
}

func TestOpenAIAccountCandidateHeap_PushPopAndInvalidType(t *testing.T) {
	h := openAIAccountCandidateHeap{}
	h.Push(openAIAccountCandidateScore{
		account:  &Account{ID: 7001},
		loadInfo: &AccountLoadInfo{LoadRate: 0, WaitingCount: 0},
		score:    1.0,
	})
	require.Equal(t, 1, h.Len())
	popped, ok := h.Pop().(openAIAccountCandidateScore)
	require.True(t, ok)
	require.Equal(t, int64(7001), popped.account.ID)
	require.Equal(t, 0, h.Len())

	require.Panics(t, func() {
		h.Push("bad_element_type")
	})
}

func TestClamp01_AllBranches(t *testing.T) {
	require.Equal(t, 0.0, clamp01(-0.2))
	require.Equal(t, 1.0, clamp01(1.3))
	require.Equal(t, 0.5, clamp01(0.5))
}

func TestCalcLoadSkewByMoments_Branches(t *testing.T) {
	require.Equal(t, 0.0, calcLoadSkewByMoments(1, 1, 1))
	// variance < 0 分支：sumSquares/count - mean^2 为负值时应钳制为 0。
	require.Equal(t, 0.0, calcLoadSkewByMoments(1, 0, 2))
	require.GreaterOrEqual(t, calcLoadSkewByMoments(6, 20, 3), 0.0)
}

func TestDefaultOpenAIAccountScheduler_ReportSwitchAndSnapshot(t *testing.T) {
	schedulerAny := newDefaultOpenAIAccountScheduler(&OpenAIGatewayService{}, nil)
	scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
	require.True(t, ok)

	ttft := 100
	scheduler.ReportResult(1001, true, &ttft)
	scheduler.ReportSwitch()
	scheduler.metrics.recordSelect(OpenAIAccountScheduleDecision{
		Layer:             openAIAccountScheduleLayerLoadBalance,
		LatencyMs:         8,
		LoadSkew:          0.5,
		StickyPreviousHit: true,
	})
	scheduler.metrics.recordSelect(OpenAIAccountScheduleDecision{
		Layer:            openAIAccountScheduleLayerSessionSticky,
		LatencyMs:        6,
		LoadSkew:         0.2,
		StickySessionHit: true,
	})

	snapshot := scheduler.SnapshotMetrics()
	require.Equal(t, int64(2), snapshot.SelectTotal)
	require.Equal(t, int64(1), snapshot.StickyPreviousHitTotal)
	require.Equal(t, int64(1), snapshot.StickySessionHitTotal)
	require.Equal(t, int64(1), snapshot.LoadBalanceSelectTotal)
	require.Equal(t, int64(1), snapshot.AccountSwitchTotal)
	require.Greater(t, snapshot.SchedulerLatencyMsAvg, 0.0)
	require.Greater(t, snapshot.StickyHitRatio, 0.0)
	require.Greater(t, snapshot.LoadSkewAvg, 0.0)
}

func TestOpenAIGatewayService_SchedulerWrappersAndDefaults(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	svc := &OpenAIGatewayService{}
	ttft := 120
	svc.ReportOpenAIAccountScheduleResult(10, true, &ttft)
	svc.RecordOpenAIAccountSwitch()
	snapshot := svc.SnapshotOpenAIAccountSchedulerMetrics()
	require.Equal(t, OpenAIAccountSchedulerMetricsSnapshot{}, snapshot)
	require.Equal(t, 7, svc.openAIWSLBTopK())
	require.Equal(t, openaiStickySessionTTL, svc.openAIWSSessionStickyTTL())

	defaultWeights := svc.openAIWSSchedulerWeights()
	require.Equal(t, 1.0, defaultWeights.Priority)
	require.Equal(t, 1.0, defaultWeights.Load)
	require.Equal(t, 0.7, defaultWeights.Queue)
	require.Equal(t, 0.8, defaultWeights.ErrorRate)
	require.Equal(t, 0.5, defaultWeights.TTFT)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 9
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 180
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 0.2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 0.3
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.4
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.5
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.6
	svcWithCfg := &OpenAIGatewayService{cfg: cfg}

	require.Equal(t, 9, svcWithCfg.openAIWSLBTopK())
	require.Equal(t, 180*time.Second, svcWithCfg.openAIWSSessionStickyTTL())
	customWeights := svcWithCfg.openAIWSSchedulerWeights()
	require.Equal(t, 0.2, customWeights.Priority)
	require.Equal(t, 0.3, customWeights.Load)
	require.Equal(t, 0.4, customWeights.Queue)
	require.Equal(t, 0.5, customWeights.ErrorRate)
	require.Equal(t, 0.6, customWeights.TTFT)
}

func TestDefaultOpenAIAccountScheduler_IsAccountTransportCompatible_Branches(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{}
	require.True(t, scheduler.isAccountTransportCompatible(nil, OpenAIUpstreamTransportAny))
	require.True(t, scheduler.isAccountTransportCompatible(nil, OpenAIUpstreamTransportHTTPSSE))
	require.False(t, scheduler.isAccountTransportCompatible(nil, OpenAIUpstreamTransportResponsesWebsocketV2))
	require.False(t, scheduler.isAccountTransportCompatible(nil, OpenAIUpstreamTransportResponsesWebsocketV2Ingress))

	cfg := newSchedulerTestOpenAIWSV2Config()
	scheduler.service = &OpenAIGatewayService{cfg: cfg}
	account := &Account{
		ID:          8801,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
	}
	require.True(t, scheduler.isAccountTransportCompatible(account, OpenAIUpstreamTransportResponsesWebsocketV2))
	require.True(t, scheduler.isAccountTransportCompatible(account, OpenAIUpstreamTransportResponsesWebsocketV2Ingress))

	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModeHTTPBridge
	require.False(t, scheduler.isAccountTransportCompatible(account, OpenAIUpstreamTransportResponsesWebsocketV2))
	require.True(t, scheduler.isAccountTransportCompatible(account, OpenAIUpstreamTransportResponsesWebsocketV2Ingress))

	account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModeOff
	require.False(t, scheduler.isAccountTransportCompatible(account, OpenAIUpstreamTransportResponsesWebsocketV2Ingress))
}

func int64PtrForTest(v int64) *int64 {
	return &v
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_StickyWeightedFallbackSkipsOutOfGroupStickyAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101081)
	otherGroupID := int64(101082)
	accounts := []Account{
		{
			ID:          38001,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    10,
			GroupIDs:    []int64{groupID},
		},
		{
			// 会话粘连绑定指向的账号已被移出请求分组（绑定 TTL 内账号改组的场景）。
			ID:          38002,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{otherGroupID},
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 2
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0.7
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 0.8
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.SessionSticky = 3
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{
		"openai:session_weighted_out_of_group": 38002,
	}}
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{38001: false, 38002: true},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_weighted_out_of_group",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	// 组内唯一候选 38001 满并发：必须返回其等待计划，绝不能把请求泄漏到组外的粘连账号 38002。
	require.Equal(t, int64(38001), selection.Account.ID)
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(38001), selection.WaitPlan.AccountID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	// 失效的粘连绑定应被清理，避免后续请求反复走同一条泄漏路径。
	require.Positive(t, cache.deletedSessions["openai:session_weighted_out_of_group"])
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_SubscriptionPriorityWaitsOnBusySubscriptionWhenRegularUnusable(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	ctx := context.Background()
	groupID := int64(101091)
	accounts := []Account{
		{
			// 订阅账号：支持 compact，但并发已满（busy-but-waitable）。
			ID:          38011,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    0,
			GroupIDs:    []int64{groupID},
			Credentials: map[string]any{"plan_type": "team"},
			Extra:       map[string]any{"openai_compact_supported": true},
		},
		{
			// 常规账号：明确不支持 compact，无法服务本次请求。
			ID:          38012,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    9,
			GroupIDs:    []int64{groupID},
			Extra:       map[string]any{"openai_compact_supported": false},
		},
	}
	concurrencyCache := schedulerTestConcurrencyCache{
		acquireResults: map[int64]bool{38011: false, 38012: true},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true", "", "true"),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		ctx,
		&groupID,
		"",
		"session_subscription_wait",
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportAny,
		true,
	)
	// 常规池无可用候选时，忙碌的订阅账号应产生等待计划，而不是直接返回 no available accounts。
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(38011), selection.Account.ID)
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(38011), selection.WaitPlan.AccountID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
}
