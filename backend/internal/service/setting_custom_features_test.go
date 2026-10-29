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
		"smtp_host": "unchanged", SettingKeyJoinGroupEnabled: "false", SettingKeyCustomMenuItems: `[{"id":"guide","label":"Guide","visibility":"user","url":"md:guide","future_field":42}]`,
	}}}
	s := NewSettingService(repo, &config.Config{})
	require.NoError(t, s.PatchCustomFeatures(context.Background(), map[string]bool{SettingKeyJoinGroupEnabled: true}, map[string]bool{"guide": false}))
	require.Equal(t, "unchanged", repo.values["smtp_host"])
	require.Equal(t, "true", repo.values[SettingKeyJoinGroupEnabled])
	require.Contains(t, repo.values[SettingKeyCustomMenuItems], `"future_field":42`)
	require.Empty(t, filterUserVisibleMenuItems(repo.values[SettingKeyCustomMenuItems])[1:len(filterUserVisibleMenuItems(repo.values[SettingKeyCustomMenuItems]))-1])
	require.Error(t, s.PatchCustomFeatures(context.Background(), map[string]bool{"smtp_host": true}, nil))
	settings, err := s.GetCustomFeatures(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Menus[0].Enabled)
	require.True(t, settings.Flags[SettingKeyJoinGroupEnabled])
}
func TestCustomFeaturesUpgradeDefaults(t *testing.T) {
	s := NewSettingService(&customFeaturesRepo{joinGroupSettingRepoStub{values: map[string]string{SettingKeyZenxiangLiyuEnabled: "", SettingKeyJoinGroupEnabled: ""}}}, &config.Config{})
	require.True(t, s.CustomFeatureEnabled(context.Background(), SettingKeyZenxiangLiyuEnabled))
	require.False(t, s.CustomFeatureEnabled(context.Background(), SettingKeyJoinGroupEnabled))
	require.False(t, s.CustomFeatureEnabled(context.Background(), SettingKeyChannelMonitorEnabled))
}

func TestCustomFeaturesOnlyIncludesConfirmedCustomSwitches(t *testing.T) {
	restored := []string{SettingKeyChannelMonitorEnabled, SettingKeyCompactHomeEnabled, SettingKeyOpenAICodexTicketEnabled}
	repo := &customFeaturesRepo{joinGroupSettingRepoStub{values: map[string]string{
		SettingKeyZenxiangLiyuEnabled:   "true",
		SettingKeyChannelMonitorEnabled: "true",
	}}}
	s := NewSettingService(repo, &config.Config{})
	settings, err := s.GetCustomFeatures(context.Background())
	require.NoError(t, err)
	require.Len(t, settings.Flags, 4)
	require.True(t, settings.Flags[SettingKeyZenxiangLiyuEnabled])
	require.True(t, settings.Flags[SettingKeyOpenAIQualityEnabled], "upgrades preserve the legacy module by default")
	for _, key := range restored {
		require.NotContains(t, settings.Flags, key)
		require.ErrorContains(t, s.PatchCustomFeatures(context.Background(), map[string]bool{key: false}, nil), "unsupported feature")
	}
	require.Equal(t, "true", repo.values[SettingKeyChannelMonitorEnabled])
}
