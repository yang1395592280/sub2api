package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type upstreamPriceGroupingGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (s *upstreamPriceGroupingGroupRepoStub) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	if platform != PlatformOpenAI {
		return nil, nil
	}
	return append([]Group(nil), s.groups...), nil
}

func TestValidateGroupUpstreamPriceGroupingConfig(t *testing.T) {
	valid := &Group{
		Platform:                      PlatformOpenAI,
		GroupRole:                     GroupRoleStandard,
		UpstreamBalanceRefreshEnabled: true,
		UpstreamPriceGroupingEnabled:  true,
		UpstreamPriceGroupingMin:      0.01,
		UpstreamPriceGroupingMax:      0.05,
	}
	require.NoError(t, ValidateGroupUpstreamPriceGroupingConfig(valid))

	withoutRefresh := *valid
	withoutRefresh.UpstreamBalanceRefreshEnabled = false
	require.ErrorContains(t, ValidateGroupUpstreamPriceGroupingConfig(&withoutRefresh), "requires upstream balance auto refresh")

	selfHosted := *valid
	selfHosted.GroupRole = GroupRoleSelfHostedPool
	require.ErrorContains(t, ValidateGroupUpstreamPriceGroupingConfig(&selfHosted), "standard OpenAI groups")

	reversed := *valid
	reversed.UpstreamPriceGroupingMin = 0.06
	require.ErrorContains(t, ValidateGroupUpstreamPriceGroupingConfig(&reversed), "minimum cannot exceed maximum")
}

func TestValidateGroupUpstreamPriceGroupingConfig_CNProviders(t *testing.T) {
	for _, platform := range []string{PlatformKimi, PlatformDeepseek} {
		group := &Group{
			Platform:                      platform,
			UpstreamBalanceRefreshEnabled: true,
			UpstreamPriceGroupingEnabled:  true,
			UpstreamPriceGroupingMin:      0.01,
			UpstreamPriceGroupingMax:      0.05,
		}
		require.NoError(t, ValidateGroupUpstreamPriceGroupingConfig(group), platform)
	}
}

func TestValidateUpstreamPriceGroupingRejectsClosedRangeOverlap(t *testing.T) {
	existing := Group{
		ID:                            10,
		Name:                          "0.01-0.05",
		Platform:                      PlatformOpenAI,
		GroupRole:                     GroupRoleStandard,
		Status:                        StatusActive,
		UpstreamBalanceRefreshEnabled: true,
		UpstreamPriceGroupingEnabled:  true,
		UpstreamPriceGroupingMin:      0.01,
		UpstreamPriceGroupingMax:      0.05,
	}
	service := &adminServiceImpl{groupRepo: &upstreamPriceGroupingGroupRepoStub{groups: []Group{existing}}}
	candidate := &Group{
		ID:                            20,
		Name:                          "candidate",
		Platform:                      PlatformOpenAI,
		GroupRole:                     GroupRoleStandard,
		Status:                        StatusActive,
		UpstreamBalanceRefreshEnabled: true,
		UpstreamPriceGroupingEnabled:  true,
		UpstreamPriceGroupingMin:      0.05,
		UpstreamPriceGroupingMax:      0.08,
	}

	err := service.validateUpstreamPriceGrouping(context.Background(), candidate)
	require.ErrorContains(t, err, "overlaps")

	candidate.UpstreamPriceGroupingMin = 0.06
	require.NoError(t, service.validateUpstreamPriceGrouping(context.Background(), candidate))
}

func TestSanitizeSelfHostedPoolClearsUpstreamPriceGrouping(t *testing.T) {
	group := &Group{
		Platform:                      PlatformOpenAI,
		GroupRole:                     GroupRoleSelfHostedPool,
		UpstreamBalanceRefreshEnabled: true,
		UpstreamPriceGroupingEnabled:  true,
		UpstreamPriceGroupingMin:      0.01,
		UpstreamPriceGroupingMax:      0.05,
	}

	sanitizeSelfHostedPoolGroup(group)

	require.False(t, group.UpstreamBalanceRefreshEnabled)
	require.False(t, group.UpstreamPriceGroupingEnabled)
	require.Zero(t, group.UpstreamPriceGroupingMin)
	require.Zero(t, group.UpstreamPriceGroupingMax)
}
