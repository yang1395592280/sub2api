package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type customFeaturesRepo struct{ joinGroupSettingRepoStub }

func (r *customFeaturesRepo) GetValue(_ context.Context, k string) (string, error) {
	return r.values[k], nil
}
func TestCustomFeaturesPatchPreservesUnrelatedSettingsAndMenuFields(t *testing.T) {
	repo := &customFeaturesRepo{joinGroupSettingRepoStub{values: map[string]string{
		"smtp_host": "unchanged", SettingKeyChannelMonitorEnabled: "false", SettingKeyCustomMenuItems: `[{"id":"guide","label":"Guide","visibility":"user","url":"md:guide","future_field":42}]`,
	}}}
	s := NewSettingService(repo, &config.Config{})
	require.NoError(t, s.PatchCustomFeatures(context.Background(), map[string]bool{SettingKeyMonitorPelicanEnabled: true}, map[string]bool{"guide": false}))
	require.Equal(t, "unchanged", repo.values["smtp_host"])
	require.Equal(t, "false", repo.values[SettingKeyChannelMonitorEnabled])
	require.Contains(t, repo.values[SettingKeyCustomMenuItems], `"future_field":42`)
	require.Empty(t, filterUserVisibleMenuItems(repo.values[SettingKeyCustomMenuItems])[1:len(filterUserVisibleMenuItems(repo.values[SettingKeyCustomMenuItems]))-1])
	require.Error(t, s.PatchCustomFeatures(context.Background(), map[string]bool{"smtp_host": true}, nil))
	settings, err := s.GetCustomFeatures(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Menus[0].Enabled)
	require.True(t, settings.Flags[SettingKeyMonitorPelicanEnabled])
}
func TestCustomFeaturesUpgradeDefaults(t *testing.T) {
	s := NewSettingService(&customFeaturesRepo{joinGroupSettingRepoStub{values: map[string]string{SettingKeyZenxiangLiyuEnabled: "", SettingKeyMonitorUserVisible: ""}}}, &config.Config{})
	require.True(t, s.CustomFeatureEnabled(context.Background(), SettingKeyZenxiangLiyuEnabled))
	require.True(t, s.CustomFeatureEnabled(context.Background(), SettingKeyMonitorUserVisible))
	require.False(t, s.CustomFeatureEnabled(context.Background(), SettingKeyMonitorPelicanEnabled))
	require.False(t, s.CustomFeatureEnabled(context.Background(), SettingKeyMonitorPelicanUserVisible))
}

func TestCustomFeaturesCodexTicketRespectsConfigFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICodexTicket.Enabled = true
	repo := &customFeaturesRepo{joinGroupSettingRepoStub{values: map[string]string{}}}
	s := NewSettingService(repo, cfg)
	settings, err := s.GetCustomFeatures(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Flags[SettingKeyOpenAICodexTicketEnabled])
	require.True(t, s.CustomFeatureEnabled(context.Background(), SettingKeyOpenAICodexTicketEnabled))
	repo.values[SettingKeyOpenAICodexTicketEnabled] = "false"
	settings, err = s.GetCustomFeatures(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Flags[SettingKeyOpenAICodexTicketEnabled])
	require.False(t, s.CustomFeatureEnabled(context.Background(), SettingKeyOpenAICodexTicketEnabled))
}
