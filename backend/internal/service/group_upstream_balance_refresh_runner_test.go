package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type groupUpstreamRefreshGroupRepoStub struct {
	GroupRepository
	groups []Group
	calls  int
	err    error
}

func (r *groupUpstreamRefreshGroupRepoStub) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return append([]Group(nil), r.groups...), nil
}

type groupUpstreamRefreshAccountRepoStub struct {
	AccountRepository
	accounts      map[int64][]Account
	accountErrs   map[int64]error
	panicGroups   map[int64]bool
	listCalls     []int64
	extraUpdates  map[int64]map[string]any
	extraHistory  map[int64][]map[string]any
	onUpdateExtra func(context.Context, int64, map[string]any) error
}

func (r *groupUpstreamRefreshAccountRepoStub) ListUpstreamBalanceRefreshCandidatesByGroupID(_ context.Context, groupID int64, _ int) ([]Account, error) {
	r.listCalls = append(r.listCalls, groupID)
	if r.panicGroups[groupID] {
		panic("list accounts boom")
	}
	if err := r.accountErrs[groupID]; err != nil {
		return nil, err
	}
	return append([]Account(nil), r.accounts[groupID]...), nil
}

func (r *groupUpstreamRefreshAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if r.onUpdateExtra != nil {
		if err := r.onUpdateExtra(ctx, id, updates); err != nil {
			return err
		}
	}
	if r.extraUpdates == nil {
		r.extraUpdates = map[int64]map[string]any{}
	}
	if r.extraHistory == nil {
		r.extraHistory = map[int64][]map[string]any{}
	}
	copied := make(map[string]any, len(updates))
	for k, v := range updates {
		copied[k] = v
	}
	r.extraUpdates[id] = copied
	r.extraHistory[id] = append(r.extraHistory[id], copied)
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

func TestGroupUpstreamBalanceRefreshRunner_RefreshesSharedAccountOnceAndFansOutPriceGuards(t *testing.T) {
	groups := []Group{
		{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.08},
		{ID: 20, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.09},
	}
	price := 0.06
	account := Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: groups}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{
		10: {account},
		20: {account},
	}}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, refresher)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{42}, refresher.calls)
	require.Len(t, accountRepo.extraHistory[42], 2)
	require.Equal(t, int64(10), accountRepo.extraHistory[42][0]["upstream_price_guard_group_id"])
	require.Equal(t, int64(20), accountRepo.extraHistory[42][1]["upstream_price_guard_group_id"])
}

func TestGroupUpstreamBalanceRefreshRunner_LeaderSkipsCycleAndReleasesOwnedLease(t *testing.T) {
	groupRepo := &groupUpstreamRefreshGroupRepoStub{}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{}
	refresher := &groupUpstreamBalanceStub{}
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: false}
	runner := newGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, refresher, lock, nil)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)

	require.Zero(t, groupRepo.calls)
	require.Equal(t, []string{groupUpstreamBalanceRefreshLeaderLockKey}, lock.keys)
	require.Empty(t, lock.releases)

	lock.acquire = true
	runner.runOnce(context.Background(), now)

	require.Equal(t, 1, groupRepo.calls)
	require.Len(t, lock.owners, 2)
	require.NotEmpty(t, lock.owners[1])
	require.Equal(t, lock.owners[0], lock.owners[1], "owner must remain stable for the runner lifetime")
	require.Equal(t, []string{groupUpstreamBalanceRefreshLeaderLockKey + ":" + lock.owners[1]}, lock.releases)
}

func TestGroupUpstreamBalanceRefreshRunner_LeaderCacheErrorFallsBackToDBAdvisoryLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	lockID := hashAdvisoryLockID(groupUpstreamBalanceRefreshLeaderLockKey)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(lockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(lockID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	groupRepo := &groupUpstreamRefreshGroupRepoStub{}
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{err: errors.New("redis unavailable")}
	runner := newGroupUpstreamBalanceRefreshRunner(
		groupRepo, &groupUpstreamRefreshAccountRepoStub{}, &groupUpstreamBalanceStub{}, lock, db,
	)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Equal(t, 1, groupRepo.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGroupUpstreamBalanceRefreshRunner_JitterDelayIsDeterministicAndBounded(t *testing.T) {
	interval := time.Minute
	require.Equal(t, 54*time.Second, nextGroupUpstreamBalanceRefreshDelay(interval, func(int64) int64 { return 0 }))
	require.Equal(t, interval, nextGroupUpstreamBalanceRefreshDelay(interval, func(int64) int64 { return int64(6 * time.Second) }))
	require.Equal(t, 66*time.Second, nextGroupUpstreamBalanceRefreshDelay(interval, func(n int64) int64 { return n - 1 }))
	require.Equal(t, interval, nextGroupUpstreamBalanceRefreshDelay(interval, nil))
}

func TestGroupUpstreamBalanceRefreshRunner_OnlyDueGroupsJoinSharedAccountPlan(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.08},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 120, UpstreamPriceMaxMultiplier: 0.09},
	}
	price := 0.06
	account := Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: groups}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{10: {account}, 20: {account}}}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, refresher)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	refresher.calls = nil
	accountRepo.extraHistory = nil
	runner.runOnce(context.Background(), now.Add(2*time.Minute))

	require.Equal(t, []int64{42}, refresher.calls)
	require.Len(t, accountRepo.extraHistory[42], 1)
	require.Equal(t, int64(20), accountRepo.extraHistory[42][0]["upstream_price_guard_group_id"])
}

func TestGroupUpstreamBalanceRefreshRunner_LeaderLeaseCoversPriceGuards(t *testing.T) {
	group := Group{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.08}
	price := 0.06
	account := Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	guardStarted := make(chan struct{}, 1)
	releaseGuard := make(chan struct{})
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {account}},
		onUpdateExtra: func(context.Context, int64, map[string]any) error {
			guardStarted <- struct{}{}
			<-releaseGuard
			return nil
		},
	}
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: true}
	runner := newGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{groups: []Group{group}},
		accountRepo,
		&groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}},
		lock,
		nil,
	)
	done := make(chan struct{})
	go func() {
		runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))
		close(done)
	}()

	select {
	case <-guardStarted:
	case <-time.After(time.Second):
		t.Fatal("price guard did not start")
	}
	lock.mu.Lock()
	require.Empty(t, lock.releases, "leader lease must remain held while a price guard is running")
	lock.mu.Unlock()
	close(releaseGuard)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh cycle did not finish")
	}
	require.Len(t, lock.releases, 1)
}

func TestGroupUpstreamBalanceRefreshRunner_ListAccountPanicContinuesLaterGroupsAndRetries(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts:    map[int64][]Account{20: {{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}}},
		panicGroups: map[int64]bool{10: true},
	}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: {ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}}}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	require.NotPanics(t, func() { runner.runOnce(context.Background(), now) })
	require.Equal(t, []int64{42}, refresher.calls)
	delete(accountRepo.panicGroups, 10)
	accountRepo.listCalls = nil
	runner.runOnce(context.Background(), now.Add(time.Minute))
	require.Equal(t, []int64{10}, accountRepo.listCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_StopCancelsInFlightRefresh(t *testing.T) {
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	refresher := &groupUpstreamBalanceStub{
		onRefresh: func(ctx context.Context, _ int64) {
			started <- struct{}{}
			<-ctx.Done()
			canceled <- struct{}{}
		},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{groups: []Group{{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600}}},
		&groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{10: {{ID: 42}}}},
		refresher,
	)
	runner.Start()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	stopped := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopped)
	}()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the in-flight refresh")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not wait for the refresh loop")
	}
}

func TestGroupUpstreamBalanceRefreshRunner_ListGroupsErrorReleasesLeaderLease(t *testing.T) {
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: true}
	runner := newGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{err: errors.New("list groups failed")},
		&groupUpstreamRefreshAccountRepoStub{},
		&groupUpstreamBalanceStub{},
		lock,
		nil,
	)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Len(t, lock.releases, 1)
}

func TestGroupUpstreamBalanceRefreshRunner_ListAccountsErrorPreservesGroupIntervalAndContinues(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts:    map[int64][]Account{20: {{ID: 42}}},
		accountErrs: map[int64]error{10: errors.New("list accounts failed")},
	}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: {ID: 42}}}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{10, 20}, accountRepo.listCalls)
	require.Equal(t, []int64{42}, refresher.calls)
}

func TestGroupUpstreamBalanceRefreshRunner_PriceGuardErrorContinuesSharedAccountFanout(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	account := Account{ID: 42}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {account}, 20: {account}},
		onUpdateExtra: func(_ context.Context, _ int64, updates map[string]any) error {
			if updates["upstream_price_guard_group_id"] == int64(10) {
				return errors.New("guard write failed")
			}
			return nil
		},
	}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{42}, refresher.calls)
	require.Len(t, accountRepo.extraHistory[42], 1)
	require.Equal(t, int64(20), accountRepo.extraHistory[42][0]["upstream_price_guard_group_id"])
}

func TestGroupUpstreamBalanceRefreshRunner_StartAndStopAreIdempotent(t *testing.T) {
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{},
		&groupUpstreamRefreshAccountRepoStub{},
		&groupUpstreamBalanceStub{},
	)
	runner.Start()
	runner.Start()
	require.NotPanics(t, func() {
		runner.Stop()
		runner.Stop()
	})
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
