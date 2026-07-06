package service

import (
	"context"
	"errors"
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
	errs      map[int64]error
	panicIDs  map[int64]bool
	calls     []int64
	onRefresh func(context.Context, int64)
}

func (s *groupUpstreamBalanceStub) Refresh(ctx context.Context, accountID int64) (*Account, error) {
	if s.onRefresh != nil {
		s.onRefresh(ctx, accountID)
	}
	s.calls = append(s.calls, accountID)
	if s.panicIDs[accountID] {
		panic("boom")
	}
	if err := s.errs[accountID]; err != nil {
		return nil, err
	}
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

func TestGroupUpstreamBalanceRefreshRunner_StopsCurrentBatchOnContextCancel(t *testing.T) {
	groupRepo := &groupUpstreamRefreshGroupRepoStub{
		groups: []Group{
			{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
			{ID: 11, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
		},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{
			10: {{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, {ID: 21, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
			11: {{ID: 30, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	balance := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{
			20: {ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			21: {ID: 21, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			30: {ID: 30, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		},
		onRefresh: func(_ context.Context, accountID int64) {
			if accountID == 20 {
				cancel()
			}
		},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)

	runner.runOnce(ctx, time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{20}, balance.calls)
}

func TestGroupUpstreamBalanceRefreshRunner_RecoversFromPanicAndContinuesNextRun(t *testing.T) {
	group := Group{
		ID:                                    10,
		Status:                                StatusActive,
		UpstreamBalanceRefreshEnabled:         true,
		UpstreamBalanceRefreshIntervalSeconds: 1,
		UpstreamPriceMaxMultiplier:            0.08,
	}
	price := 0.06
	account := Account{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: []Group{group}}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {account}},
	}
	balance := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{20: &account},
		panicIDs:  map[int64]bool{20: true},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)

	require.NotPanics(t, func() {
		runner.runOnce(context.Background(), now)
	})
	require.Equal(t, []int64{20}, balance.calls)

	delete(balance.panicIDs, 20)
	balance.calls = nil

	runner.runOnce(context.Background(), now.Add(2*time.Second))

	require.Equal(t, []int64{20}, balance.calls)
	require.Equal(t, "ok", accountRepo.extraUpdates[20]["upstream_price_guard_status"])
}

func TestGroupUpstreamBalanceRefreshRunner_PanicInOneGroupContinuesLaterGroupsAndRetriesPanickedGroup(t *testing.T) {
	groupRepo := &groupUpstreamRefreshGroupRepoStub{
		groups: []Group{
			{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
			{ID: 11, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
		},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{
			10: {{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
			11: {{ID: 30, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
		},
	}
	balance := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{
			20: {ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			30: {ID: 30, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		},
		panicIDs: map[int64]bool{20: true},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)

	require.NotPanics(t, func() {
		runner.runOnce(context.Background(), now)
	})
	require.Equal(t, []int64{20, 30}, balance.calls)

	delete(balance.panicIDs, 20)
	balance.calls = nil

	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{20}, balance.calls)
}

func TestGroupUpstreamBalanceRefreshRunner_RefreshErrorStillContinuesSameGroup(t *testing.T) {
	group := Group{
		ID:                                    10,
		Status:                                StatusActive,
		UpstreamBalanceRefreshEnabled:         true,
		UpstreamBalanceRefreshIntervalSeconds: 600,
	}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: []Group{group}}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{
			10: {
				{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
				{ID: 21, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			},
		},
	}
	balance := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{
			21: {ID: 21, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		},
		errs: map[int64]error{
			20: errors.New("refresh failed"),
		},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)

	runner.runOnce(context.Background(), time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{20, 21}, balance.calls)
}
