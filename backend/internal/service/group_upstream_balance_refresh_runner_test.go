package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type groupUpstreamRefreshGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (r *groupUpstreamRefreshGroupRepoStub) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	return append([]Group(nil), r.groups...), nil
}

type groupUpstreamRefreshAccountRepoStub struct {
	AccountRepository
	accounts     map[int64][]Account
	extraUpdates map[int64]map[string]any
}

func (r *groupUpstreamRefreshAccountRepoStub) ListUpstreamBalanceRefreshCandidatesByGroupID(_ context.Context, groupID int64, _ int) ([]Account, error) {
	return append([]Account(nil), r.accounts[groupID]...), nil
}

func (r *groupUpstreamRefreshAccountRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	if r.extraUpdates == nil {
		r.extraUpdates = map[int64]map[string]any{}
	}
	copied := make(map[string]any, len(updates))
	for k, v := range updates {
		copied[k] = v
	}
	r.extraUpdates[id] = copied
	return nil
}

func (r *groupUpstreamRefreshAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}

func (r *groupUpstreamRefreshAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	return nil
}

type groupUpstreamBalanceStub struct {
	refreshed map[int64]*Account
	calls     []int64
}

func (s *groupUpstreamBalanceStub) Refresh(_ context.Context, accountID int64) (*Account, error) {
	s.calls = append(s.calls, accountID)
	return s.refreshed[accountID], nil
}

func TestGroupUpstreamBalanceRefreshRunner_RunOnceRefreshesGroupAccounts(t *testing.T) {
	group := Group{
		ID:                                    10,
		Status:                                StatusActive,
		UpstreamBalanceRefreshEnabled:         true,
		UpstreamBalanceRefreshIntervalSeconds: 600,
		UpstreamPriceMaxMultiplier:            0.08,
	}
	price := 0.06
	account := Account{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: []Group{group}}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {account}},
	}
	balance := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{20: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)

	runner.runOnce(context.Background(), time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{20}, balance.calls)
	require.Equal(t, "ok", accountRepo.extraUpdates[20]["upstream_price_guard_status"])
}

func TestGroupUpstreamBalanceRefreshRunner_RespectsGroupInterval(t *testing.T) {
	group := Group{
		ID:                                    10,
		Status:                                StatusActive,
		UpstreamBalanceRefreshEnabled:         true,
		UpstreamBalanceRefreshIntervalSeconds: 600,
	}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: []Group{group}}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}}},
	}
	balance := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{
		20: {ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{20}, balance.calls)
}
