package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToService_PreservesMessagesDispatchModelConfig(t *testing.T) {
	group := &dbent.Group{
		ID:                    1,
		Name:                  "openai-dispatch",
		Platform:              service.PlatformOpenAI,
		Status:                service.StatusActive,
		SubscriptionType:      service.SubscriptionTypeStandard,
		RateMultiplier:        1,
		AllowMessagesDispatch: true,
		DefaultMappedModel:    "gpt-5.4",
		VideoModelPrices: map[string]map[string]float64{
			service.VideoPriceFamilyGrokImagineVideo15: {service.VideoBillingResolution720P: 0.14},
		},
		MessagesDispatchModelConfig: service.OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:   "gpt-5.4-nano",
			SonnetMappedModel: "gpt-5.3-codex",
			HaikuMappedModel:  "gpt-5.4-mini",
			ExactModelMappings: map[string]string{
				"claude-sonnet-4.5": "gpt-5.4-nano",
			},
		},
	}

	got := groupEntityToService(group)
	require.NotNil(t, got)
	require.Equal(t, group.MessagesDispatchModelConfig, got.MessagesDispatchModelConfig)
	require.Equal(t, group.VideoModelPrices, got.VideoModelPrices)
}

func TestGroupEntityToService_PreservesAutoCheapestSchedulingFlag(t *testing.T) {
	group := &dbent.Group{
		ID:                          2,
		Name:                        "openai-auto-cheapest",
		Platform:                    service.PlatformOpenAI,
		Status:                      service.StatusActive,
		RateMultiplier:              0.15,
		AllowAutoCheapestScheduling: true,
	}

	got := groupEntityToService(group)
	require.NotNil(t, got)
	require.True(t, got.AllowAutoCheapestScheduling,
		"group mapper must preserve allow_auto_cheapest_scheduling for automatic group selection")
}

func TestGroupRepository_PersistsAutoSchedulingAndUpstreamGuardFlags_SQLite(t *testing.T) {
	_, client := newAPIKeyRepoSQLite(t)
	repo := newGroupRepositoryWithSQL(client, nil)
	ctx := context.Background()

	group := &service.Group{
		Name:                                  "group-auto-scheduling-roundtrip",
		Platform:                              service.PlatformOpenAI,
		Status:                                service.StatusActive,
		SubscriptionType:                      service.SubscriptionTypeStandard,
		RateMultiplier:                        0.2,
		OpenAIAutoSchedulerEnabled:            true,
		AllowAutoCheapestScheduling:           false,
		UpstreamBalanceRefreshEnabled:         true,
		UpstreamBalanceRefreshIntervalSeconds: 777,
		UpstreamPriceMaxMultiplier:            1.25,
		UpstreamPriceGroupingEnabled:          true,
		UpstreamPriceGroupingMin:              0.1,
		UpstreamPriceGroupingMax:              0.3,
	}
	require.NoError(t, repo.Create(ctx, group))

	got, err := repo.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.True(t, got.OpenAIAutoSchedulerEnabled)
	require.False(t, got.AllowAutoCheapestScheduling)
	require.True(t, got.UpstreamBalanceRefreshEnabled)
	require.Equal(t, 777, got.UpstreamBalanceRefreshIntervalSeconds)
	require.InDelta(t, 1.25, got.UpstreamPriceMaxMultiplier, 1e-12)
	require.True(t, got.UpstreamPriceGroupingEnabled)
	require.InDelta(t, 0.1, got.UpstreamPriceGroupingMin, 1e-12)
	require.InDelta(t, 0.3, got.UpstreamPriceGroupingMax, 1e-12)

	group.OpenAIAutoSchedulerEnabled = false
	group.AllowAutoCheapestScheduling = true
	group.UpstreamBalanceRefreshEnabled = false
	group.UpstreamPriceGroupingEnabled = false
	require.NoError(t, repo.Update(ctx, group))

	got, err = repo.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.False(t, got.OpenAIAutoSchedulerEnabled)
	require.True(t, got.AllowAutoCheapestScheduling)
	require.False(t, got.UpstreamBalanceRefreshEnabled)
	require.False(t, got.UpstreamPriceGroupingEnabled)
}

func TestAPIKeyRepository_GetByKeyForAuth_PreservesMessagesDispatchModelConfig_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-dispatch-unit@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-dispatch-unit").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		SetAllowMessagesDispatch(true).
		SetAllowAutoCheapestScheduling(true).
		SetDefaultMappedModel("gpt-5.4").
		SetMessagesDispatchModelConfig(service.OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:   "gpt-5.4-nano",
			SonnetMappedModel: "gpt-5.3-codex",
			HaikuMappedModel:  "gpt-5.4-mini",
			ExactModelMappings: map[string]string{
				"claude-sonnet-4.5": "gpt-5.4-nano",
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-dispatch-unit",
		Name:    "Dispatch Key Unit",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, key.Name, got.Name)
	require.NotNil(t, got.Group)
	require.Equal(t, group.MessagesDispatchModelConfig, got.Group.MessagesDispatchModelConfig)
	require.True(t, got.Group.AllowAutoCheapestScheduling,
		"auth group projection must preserve allow_auto_cheapest_scheduling")
}

func TestAPIKeyRepository_GetByKeyForAuth_PreservesSelfHostedPoolMetadata_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-self-hosted-pool@test.com")

	pool, err := client.Group.Create().
		SetName("self-hosted-pool").
		SetPlatform(service.PlatformOpenAI).
		SetGroupRole(service.GroupRoleSelfHostedPool).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName("standard-with-self-hosted-pool").
		SetPlatform(service.PlatformOpenAI).
		SetGroupRole(service.GroupRoleStandard).
		SetSelfHostedPoolGroupID(pool.ID).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(0.3).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-self-hosted-pool",
		Name:    "Self-hosted Pool Key",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.Equal(t, service.GroupRoleStandard, got.Group.GroupRole)
	require.NotNil(t, got.Group.SelfHostedPoolGroupID)
	require.Equal(t, pool.ID, *got.Group.SelfHostedPoolGroupID)
}

func TestAPIKeyRepository_GetByKeyForAuth_PreservesUpstreamPriceGuardConfig_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-price-guard-unit@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-price-guard-unit").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		SetUpstreamBalanceRefreshEnabled(true).
		SetUpstreamBalanceRefreshIntervalSeconds(777).
		SetUpstreamPriceMaxMultiplier(2.75).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-price-guard-unit",
		Name:    "Price Guard Key Unit",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.True(t, got.Group.UpstreamBalanceRefreshEnabled)
	require.Equal(t, 777, got.Group.UpstreamBalanceRefreshIntervalSeconds)
	require.Equal(t, 2.75, got.Group.UpstreamPriceMaxMultiplier)
}

func TestAPIKeyRepository_GetByKeyForAuth_PreservesProfitControlConfig_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-profit-control@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-profit-control").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(0.06).
		SetProfitControlEnabled(true).
		SetProfitMinMargin(0.2).
		SetProfitSafetyBuffer(0.05).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-profit-control",
		Name:    "Profit Control Key",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.True(t, got.Group.ProfitControlEnabled)
	require.InDelta(t, 0.2, got.Group.ProfitMinMargin, 1e-12)
	require.InDelta(t, 0.05, got.Group.ProfitSafetyBuffer, 1e-12)
}
