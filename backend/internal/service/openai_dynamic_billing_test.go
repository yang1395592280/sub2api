package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type dynamicBillingSettingRepoStub struct {
	values map[string]string
}

func (r *dynamicBillingSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *dynamicBillingSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}
func (r *dynamicBillingSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (r *dynamicBillingSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func (r *dynamicBillingSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *dynamicBillingSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *dynamicBillingSettingRepoStub) Delete(context.Context, string) error { return nil }

func dynamicBillingTestAPIKey() *APIKey {
	return &APIKey{Group: &Group{
		Platform:                     PlatformOpenAI,
		GroupRole:                    GroupRoleStandard,
		SubscriptionType:             SubscriptionTypeStandard,
		DynamicBillingEnabled:        true,
		UpstreamPriceGroupingEnabled: true,
		UpstreamPriceGroupingMin:     0.03,
		UpstreamPriceGroupingMax:     0.12,
	}}
}

func TestResolveOpenAIDynamicBillingMultiplier_ChannelPricePlusProfit(t *testing.T) {
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)
	price := 0.04

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), dynamicBillingTestAPIKey(), &Account{ChannelPrice: &price}, settings, 1)

	require.InDelta(t, 0.07, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_GroupProfitOverridesGlobal(t *testing.T) {
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)
	price := 0.04
	groupProfit := 0.05
	apiKey := dynamicBillingTestAPIKey()
	apiKey.Group.DynamicBillingProfitMarkup = &groupProfit

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), apiKey, &Account{ChannelPrice: &price}, settings, 1)

	require.InDelta(t, 0.09, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_ZeroGroupProfitOverridesGlobal(t *testing.T) {
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)
	price := 0.04
	groupProfit := 0.0
	apiKey := dynamicBillingTestAPIKey()
	apiKey.Group.DynamicBillingProfitMarkup = &groupProfit

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), apiKey, &Account{ChannelPrice: &price}, settings, 1)

	require.InDelta(t, 0.04, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_CapsAtDynamicGroupMaximum(t *testing.T) {
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)
	price := 0.14

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), dynamicBillingTestAPIKey(), &Account{ChannelPrice: &price}, settings, 1)

	require.InDelta(t, 0.15, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_MissingChannelPriceUsesMaximum(t *testing.T) {
	settings := NewSettingService(&dynamicBillingSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIDynamicBillingProfitMarkup: "0.03",
	}}, nil)

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), dynamicBillingTestAPIKey(), &Account{}, settings, 1)

	require.InDelta(t, 0.15, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_StaticGroupKeepsFallback(t *testing.T) {
	apiKey := dynamicBillingTestAPIKey()
	apiKey.Group.DynamicBillingEnabled = false
	price := 0.04

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), apiKey, &Account{ChannelPrice: &price}, nil, 0.8)

	require.InDelta(t, 0.8, got, 1e-12)
}

func TestResolveOpenAIDynamicBillingMultiplier_GroupingDisabledKeepsFallback(t *testing.T) {
	apiKey := dynamicBillingTestAPIKey()
	apiKey.Group.UpstreamPriceGroupingEnabled = false
	price := 0.04

	got := ResolveOpenAIDynamicBillingMultiplier(context.Background(), apiKey, &Account{ChannelPrice: &price}, nil, 0.8)

	require.InDelta(t, 0.8, got, 1e-12)
}

func TestValidateDynamicBillingConfigRequiresGroupingRange(t *testing.T) {
	group := dynamicBillingTestAPIKey().Group
	group.UpstreamPriceGroupingEnabled = false

	err := validateDynamicBillingConfig(group)

	require.Error(t, err)
}

func TestValidateDynamicBillingConfigRejectsNegativeGroupProfit(t *testing.T) {
	group := dynamicBillingTestAPIKey().Group
	groupProfit := -0.01
	group.DynamicBillingProfitMarkup = &groupProfit

	err := validateDynamicBillingConfig(group)

	require.Error(t, err)
}
