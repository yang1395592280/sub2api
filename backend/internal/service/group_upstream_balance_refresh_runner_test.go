package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
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
	tempReasons   map[int64]string
	tempUntils    map[int64]*time.Time
	getByIDCalls  []int64
	getByIDErrs   map[int64]error
	getByIDNils   map[int64]bool
	getByIDPanics map[int64]bool
}

func (r *groupUpstreamRefreshAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	r.getByIDCalls = append(r.getByIDCalls, id)
	if r.getByIDPanics[id] {
		panic("get account state boom")
	}
	if err := r.getByIDErrs[id]; err != nil {
		return nil, err
	}
	if r.getByIDNils[id] {
		return nil, nil
	}
	return &Account{ID: id, TempUnschedulableReason: r.tempReasons[id], TempUnschedulableUntil: r.tempUntils[id]}, nil
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

func (r *groupUpstreamRefreshAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	if existing := r.tempUntils[id]; existing != nil && !existing.Before(until) {
		return nil
	}
	if r.tempReasons == nil {
		r.tempReasons = map[int64]string{}
	}
	if r.tempUntils == nil {
		r.tempUntils = map[int64]*time.Time{}
	}
	untilCopy := until
	r.tempReasons[id] = reason
	r.tempUntils[id] = &untilCopy
	return nil
}

func (r *groupUpstreamRefreshAccountRepoStub) ClearTempUnschedulable(_ context.Context, id int64) error {
	delete(r.tempReasons, id)
	delete(r.tempUntils, id)
	return nil
}

type groupUpstreamBalanceStub struct {
	refreshed map[int64]*Account
	errs      map[int64]error
	panicIDs  map[int64]bool
	calls     []int64
	onRefresh func(context.Context, int64)
}

type groupUpstreamRefreshSettingRepoStub struct {
	SettingRepository
	values   map[string]string
	getErr   error
	setErr   error
	getCalls [][]string
	setCalls map[string]string
}

func (r *groupUpstreamRefreshSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.getCalls = append(r.getCalls, append([]string(nil), keys...))
	if r.getErr != nil {
		return nil, r.getErr
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *groupUpstreamRefreshSettingRepoStub) Set(_ context.Context, key, value string) error {
	if r.setCalls == nil {
		r.setCalls = map[string]string{}
	}
	r.setCalls[key] = value
	if r.setErr != nil {
		return r.setErr
	}
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
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

func TestGroupUpstreamBalanceRefreshRunner_LeaderLeaseOutlivesMaxCycle(t *testing.T) {
	require.Greater(t, groupUpstreamBalanceRefreshLeaderLockTTL, groupUpstreamBalanceRefreshMaxCycleRuntime)
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

func TestGroupUpstreamBalanceRefreshRunner_PriceGuardPanicOnlyFailsThatMembership(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	account := Account{ID: 42}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts: map[int64][]Account{10: {account}, 20: {account}},
		onUpdateExtra: func(_ context.Context, _ int64, updates map[string]any) error {
			if updates["upstream_price_guard_group_id"] == int64(10) {
				panic("group 10 guard panic")
			}
			return nil
		},
	}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)

	require.NotPanics(t, func() {
		runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))
	})

	require.Equal(t, []int64{42}, refresher.calls)
	require.Len(t, accountRepo.extraHistory[42], 1)
	require.Equal(t, int64(20), accountRepo.extraHistory[42][0]["upstream_price_guard_group_id"])
	accountRepo.onUpdateExtra = nil
	accountRepo.listCalls = nil
	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 1, 0, 0, time.UTC))
	require.Equal(t, []int64{10}, accountRepo.listCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_ReloadsSharedGuardStateBetweenMemberships(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.05},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.10},
	}
	price := 0.08
	initialReason := UpstreamPriceGuardReasonPrefix + " group_id=20"
	account := Account{ID: 42, ChannelPrice: &price, TempUnschedulableReason: initialReason}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts:    map[int64][]Account{10: {account}, 20: {account}},
		tempReasons: map[int64]string{42: initialReason},
		tempUntils:  map[int64]*time.Time{},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{groups: groups},
		accountRepo,
		&groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}},
	)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Contains(t, accountRepo.tempReasons[42], "group_id=10")
	require.Equal(t, []int64{42, 42}, accountRepo.getByIDCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_ReloadsStateWhenConditionalSetUpdatesZeroRows(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	price := 0.08
	existingUntil := now.Add(48 * time.Hour)
	existingReason := UpstreamPriceGuardReasonPrefix + " group_id=20"
	account := Account{ID: 42, ChannelPrice: &price}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{
		accounts:    map[int64][]Account{10: {account}},
		tempReasons: map[int64]string{42: existingReason},
		tempUntils:  map[int64]*time.Time{42: &existingUntil},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{groups: []Group{{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.05}}},
		accountRepo,
		&groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}},
	)

	runner.runOnce(context.Background(), now)

	require.Equal(t, existingReason, account.TempUnschedulableReason)
	require.Equal(t, existingUntil, *account.TempUnschedulableUntil)
}

func TestGroupUpstreamBalanceRefreshRunner_UntrustedReloadStopsOnlyCurrentAccountFanout(t *testing.T) {
	cases := map[string]func(*groupUpstreamRefreshAccountRepoStub){
		"error": func(repo *groupUpstreamRefreshAccountRepoStub) {
			repo.getByIDErrs = map[int64]error{42: errors.New("reload failed")}
		},
		"nil": func(repo *groupUpstreamRefreshAccountRepoStub) {
			repo.getByIDNils = map[int64]bool{42: true}
		},
		"panic": func(repo *groupUpstreamRefreshAccountRepoStub) {
			repo.getByIDPanics = map[int64]bool{42: true}
		},
	}
	for name, configureFailure := range cases {
		t.Run(name, func(t *testing.T) {
			groups := []Group{
				{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.05},
				{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.10},
				{ID: 30, UpstreamBalanceRefreshIntervalSeconds: 600},
			}
			price := 0.08
			initialReason := UpstreamPriceGuardReasonPrefix + " group_id=20"
			account42 := Account{ID: 42, ChannelPrice: &price, TempUnschedulableReason: initialReason}
			account43 := Account{ID: 43}
			accountRepo := &groupUpstreamRefreshAccountRepoStub{
				accounts:    map[int64][]Account{10: {account42}, 20: {account42}, 30: {account43}},
				tempReasons: map[int64]string{42: initialReason},
				tempUntils:  map[int64]*time.Time{},
			}
			configureFailure(accountRepo)
			refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account42, 43: &account43}}
			runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)
			now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

			require.NotPanics(t, func() { runner.runOnce(context.Background(), now) })

			require.Contains(t, accountRepo.tempReasons[42], "group_id=10")
			require.Len(t, accountRepo.extraHistory[43], 1, "an untrusted account state must not stop later account plans")
			accountRepo.getByIDErrs = nil
			accountRepo.getByIDNils = nil
			accountRepo.getByIDPanics = nil
			accountRepo.listCalls = nil
			runner.runOnce(context.Background(), now.Add(time.Minute))
			require.Equal(t, []int64{10, 20}, accountRepo.listCalls)
		})
	}
}

func TestGroupUpstreamBalanceRefreshRunner_ReloadPanicIsContainedByMembership(t *testing.T) {
	account := &Account{ID: 42}
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{},
		&groupUpstreamRefreshAccountRepoStub{getByIDPanics: map[int64]bool{42: true}},
		&groupUpstreamBalanceStub{},
	)

	require.NotPanics(t, func() {
		runner.applyPriceGuardMembership(context.Background(), account, Group{ID: 10}, time.Now())
	})
}

func TestGroupUpstreamBalanceRefreshRunner_CommitsCompletedGroupBeforeLaterCancellation(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{
		10: {{ID: 42}},
		20: {{ID: 43}},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	refresher := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{42: {ID: 42}, 43: {ID: 43}},
		onRefresh: func(_ context.Context, accountID int64) {
			if accountID == 43 {
				cancel()
			}
		},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(ctx, now)
	accountRepo.listCalls = nil
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{20}, accountRepo.listCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_CommitsEmptyGroupBeforeLaterCancellation(t *testing.T) {
	groups := []Group{
		{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{20: {{ID: 43}}}}
	ctx, cancel := context.WithCancel(context.Background())
	refresher := &groupUpstreamBalanceStub{
		refreshed: map[int64]*Account{43: {ID: 43}},
		onRefresh: func(_ context.Context, accountID int64) {
			if accountID == 43 {
				cancel()
			}
		},
	}
	runner := NewGroupUpstreamBalanceRefreshRunner(&groupUpstreamRefreshGroupRepoStub{groups: groups}, accountRepo, refresher)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(ctx, now)
	accountRepo.listCalls = nil
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{20}, accountRepo.listCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_DistributedLastRunPreventsAlternatingRunnerRefresh(t *testing.T) {
	group := Group{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600}
	account := Account{ID: 42}
	settings := &groupUpstreamRefreshSettingRepoStub{values: map[string]string{}}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	newRunner := func() *GroupUpstreamBalanceRefreshRunner {
		return newGroupUpstreamBalanceRefreshRunnerWithState(
			&groupUpstreamRefreshGroupRepoStub{groups: []Group{group}},
			&groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{10: {account}}},
			refresher,
			&fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: true},
			nil,
			settings,
		)
	}
	runnerA := newRunner()
	runnerB := newRunner()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runnerA.runOnce(context.Background(), now)
	runnerB.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{42}, refresher.calls)
	require.Equal(t, now.Unix(), mustParseGroupUpstreamBalanceRefreshLastRun(t, settings.values[groupUpstreamBalanceRefreshLastRunKey(10)]))
	require.Len(t, settings.getCalls, 2)
}

func TestGroupUpstreamBalanceRefreshRunner_LoadsAllGroupLastRunsInOneBatch(t *testing.T) {
	groups := []Group{{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600}, {ID: 20, UpstreamBalanceRefreshIntervalSeconds: 600}}
	settings := &groupUpstreamRefreshSettingRepoStub{values: map[string]string{}}
	runner := newGroupUpstreamBalanceRefreshRunnerWithState(
		&groupUpstreamRefreshGroupRepoStub{groups: groups},
		&groupUpstreamRefreshAccountRepoStub{},
		&groupUpstreamBalanceStub{},
		nil, nil, settings,
	)

	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))

	require.Equal(t, [][]string{{
		groupUpstreamBalanceRefreshLastRunKey(10),
		groupUpstreamBalanceRefreshLastRunKey(20),
	}}, settings.getCalls)
}

func TestGroupUpstreamBalanceRefreshRunner_DistributedReadFailureUsesLocalFallback(t *testing.T) {
	group := Group{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600}
	account := Account{ID: 42}
	settings := &groupUpstreamRefreshSettingRepoStub{getErr: errors.New("settings read failed")}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := newGroupUpstreamBalanceRefreshRunnerWithState(
		&groupUpstreamRefreshGroupRepoStub{groups: []Group{group}},
		&groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{10: {account}}},
		refresher, nil, nil, settings,
	)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{42}, refresher.calls)
}

func TestGroupUpstreamBalanceRefreshRunner_DistributedWriteFailureKeepsLocalLastRun(t *testing.T) {
	group := Group{ID: 10, UpstreamBalanceRefreshIntervalSeconds: 600}
	account := Account{ID: 42}
	settings := &groupUpstreamRefreshSettingRepoStub{values: map[string]string{}, setErr: errors.New("settings write failed")}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := newGroupUpstreamBalanceRefreshRunnerWithState(
		&groupUpstreamRefreshGroupRepoStub{groups: []Group{group}},
		&groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{10: {account}}},
		refresher, nil, nil, settings,
	)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{42}, refresher.calls)
	require.Empty(t, settings.values)
}

func mustParseGroupUpstreamBalanceRefreshLastRun(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	return parsed
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

func TestGroupUpstreamBalanceRefreshRunner_ConcurrentStartStopIsSafe(t *testing.T) {
	runner := NewGroupUpstreamBalanceRefreshRunner(
		&groupUpstreamRefreshGroupRepoStub{},
		&groupUpstreamRefreshAccountRepoStub{},
		&groupUpstreamBalanceStub{},
	)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			runner.Start()
		}()
		go func() {
			defer wg.Done()
			runner.Stop()
		}()
	}
	wg.Wait()
	runner.Stop()
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
