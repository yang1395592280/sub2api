package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type adminGroupPriceGuardGroupRepoStub struct {
	GroupRepository
	group   *Group
	updated *Group
}

func (r *adminGroupPriceGuardGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func (r *adminGroupPriceGuardGroupRepoStub) Update(_ context.Context, group *Group) error {
	cp := *group
	r.updated = &cp
	return nil
}

type adminGroupPriceGuardAccountRepoStub struct {
	AccountRepository
	accounts  []Account
	setID     int64
	setUntil  time.Time
	setReason string
}

func (r *adminGroupPriceGuardAccountRepoStub) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func (r *adminGroupPriceGuardAccountRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}

func (r *adminGroupPriceGuardAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.setID = id
	r.setUntil = until
	r.setReason = reason
	return nil
}

func (r *adminGroupPriceGuardAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	return nil
}

func TestAdminServiceUpdateGroup_AppliesUpstreamPriceGuardImmediately(t *testing.T) {
	groupID := int64(7)
	channelPrice := 0.002
	groupRepo := &adminGroupPriceGuardGroupRepoStub{
		group: &Group{
			ID:                                    groupID,
			Name:                                  "special",
			Platform:                              PlatformOpenAI,
			Status:                                StatusActive,
			RateMultiplier:                        0.03,
			UpstreamBalanceRefreshEnabled:         true,
			UpstreamBalanceRefreshIntervalSeconds: 60,
		},
	}
	accountRepo := &adminGroupPriceGuardAccountRepoStub{
		accounts: []Account{{
			ID:           12,
			Platform:     PlatformOpenAI,
			Type:         AccountTypeAPIKey,
			ChannelPrice: &channelPrice,
		}},
	}
	maxMultiplier := 0.001
	svc := &adminServiceImpl{groupRepo: groupRepo, accountRepo: accountRepo}

	_, err := svc.UpdateGroup(context.Background(), groupID, &UpdateGroupInput{
		UpstreamPriceMaxMultiplier: &maxMultiplier,
	})

	require.NoError(t, err)
	require.NotNil(t, groupRepo.updated)
	require.Equal(t, int64(12), accountRepo.setID)
	require.True(t, accountRepo.setUntil.After(time.Now()))
	require.Contains(t, accountRepo.setReason, UpstreamPriceGuardReasonPrefix)
	require.Contains(t, accountRepo.setReason, "group_id=7")
}
