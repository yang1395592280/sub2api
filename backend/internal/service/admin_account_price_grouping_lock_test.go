//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePriceGroupingLockedGroups(t *testing.T) {
	groups := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		10: {
			ID:                           10,
			Platform:                     PlatformOpenAI,
			GroupRole:                    GroupRoleStandard,
			UpstreamPriceGroupingEnabled: true,
		},
		20: {
			ID:                           20,
			Platform:                     PlatformOpenAI,
			GroupRole:                    GroupRoleSelfHostedPool,
			UpstreamPriceGroupingEnabled: true,
		},
	}}
	svc := &adminServiceImpl{groupRepo: groups}

	locked, err := svc.validatePriceGroupingLockedGroups(
		context.Background(),
		PlatformOpenAI,
		[]int64{10},
		[]int64{10, 10},
	)
	require.NoError(t, err)
	require.Equal(t, []int64{10}, locked)

	_, err = svc.validatePriceGroupingLockedGroups(
		context.Background(),
		PlatformOpenAI,
		[]int64{10},
		[]int64{20},
	)
	require.ErrorContains(t, err, "must also be present in group_ids")

	_, err = svc.validatePriceGroupingLockedGroups(
		context.Background(),
		PlatformAnthropic,
		[]int64{10},
		[]int64{10},
	)
	require.ErrorContains(t, err, "only support OpenAI accounts")

	_, err = svc.validatePriceGroupingLockedGroups(
		context.Background(),
		PlatformOpenAI,
		[]int64{20},
		[]int64{20},
	)
	require.ErrorContains(t, err, "standard OpenAI groups")
}

func TestAdminService_UpdateAccountPersistsPriceGroupingLocks(t *testing.T) {
	repo := &accountRepoStubForBulkUpdate{getByIDAccounts: map[int64]*Account{
		7: {
			ID:       7,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
			Extra:    map[string]any{},
			GroupIDs: []int64{10, 20},
		},
	}}
	groupRepo := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		10: {
			ID:       10,
			Platform: PlatformOpenAI,
			GroupRole: GroupRoleStandard,
		},
		20: {
			ID:                           20,
			Platform:                     PlatformOpenAI,
			GroupRole:                    GroupRoleStandard,
			UpstreamPriceGroupingEnabled: true,
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}
	groupIDs := []int64{10, 20}
	lockedGroupIDs := []int64{20}

	_, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{
		GroupIDs:                    &groupIDs,
		PriceGroupingLockedGroupIDs: &lockedGroupIDs,
		SkipMixedChannelCheck:       true,
	})

	require.NoError(t, err)
	require.Equal(t, []int64{10, 20}, repo.bindGroupsByAccount[7])
	require.Equal(t, []int64{20}, repo.lockedGroupsByAccount[7])
}
