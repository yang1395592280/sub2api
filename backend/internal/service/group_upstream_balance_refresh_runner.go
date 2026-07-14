package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	groupUpstreamBalanceRefreshScanInterval    = time.Minute
	groupUpstreamBalanceRefreshCandidateLimit  = 0
	groupUpstreamBalanceRefreshLeaderLockKey   = "group-upstream-balance-refresh"
	groupUpstreamBalanceRefreshMaxCycleRuntime = 30 * time.Minute
	groupUpstreamBalanceRefreshLeaderLockTTL   = 35 * time.Minute
	groupUpstreamBalanceRefreshJitterPercent   = 10
	groupUpstreamBalanceRefreshLastRunPrefix   = "group-upstream-balance-refresh:last-run:"
)

type groupUpstreamBalanceRefresher interface {
	Refresh(ctx context.Context, accountID int64) (*Account, error)
}

type groupUpstreamBalanceRefreshStateRepository interface {
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
}

type GroupUpstreamBalanceRefreshRunner struct {
	groupRepo   GroupRepository
	accountRepo AccountRepository
	refresher   groupUpstreamBalanceRefresher
	lockCache   LeaderLockCache
	db          *sql.DB
	stateRepo   groupUpstreamBalanceRefreshStateRepository
	owner       string
	randInt64   func(int64) int64

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	mu        sync.Mutex
	stopped   bool
	parentCtx context.Context
	cancel    context.CancelFunc
	lastRun   map[int64]time.Time
}

type groupUpstreamBalanceRefreshPlanItem struct {
	accountID int64
	groups    []Group
}

type groupUpstreamBalanceRefreshGroupState struct {
	pending int
	failed  bool
}

type groupUpstreamBalanceRefreshMembershipResult struct {
	groupID int64
	failed  bool
}

func NewGroupUpstreamBalanceRefreshRunner(groupRepo GroupRepository, accountRepo AccountRepository, refresher groupUpstreamBalanceRefresher) *GroupUpstreamBalanceRefreshRunner {
	return newGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, refresher, nil, nil)
}

func newGroupUpstreamBalanceRefreshRunner(
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	refresher groupUpstreamBalanceRefresher,
	lockCache LeaderLockCache,
	db *sql.DB,
) *GroupUpstreamBalanceRefreshRunner {
	return newGroupUpstreamBalanceRefreshRunnerWithState(groupRepo, accountRepo, refresher, lockCache, db, nil)
}

func newGroupUpstreamBalanceRefreshRunnerWithState(
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	refresher groupUpstreamBalanceRefresher,
	lockCache LeaderLockCache,
	db *sql.DB,
	stateRepo groupUpstreamBalanceRefreshStateRepository,
) *GroupUpstreamBalanceRefreshRunner {
	parentCtx, cancel := context.WithCancel(context.Background())
	return &GroupUpstreamBalanceRefreshRunner{
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
		refresher:   refresher,
		lockCache:   lockCache,
		db:          db,
		stateRepo:   stateRepo,
		owner:       uuid.NewString(),
		randInt64:   rand.Int64N,
		parentCtx:   parentCtx,
		cancel:      cancel,
		lastRun:     map[int64]time.Time{},
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) Start() {
	if r == nil || r.groupRepo == nil || r.accountRepo == nil || r.refresher == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.startOnce.Do(func() {
		r.wg.Add(1)
		go r.loop()
	})
}

func (r *GroupUpstreamBalanceRefreshRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		r.mu.Lock()
		r.stopped = true
		if r.cancel != nil {
			r.cancel()
		}
		r.mu.Unlock()
	})
	r.wg.Wait()
}

func (r *GroupUpstreamBalanceRefreshRunner) loop() {
	defer r.wg.Done()

	ctx := r.parentCtx
	if ctx == nil {
		ctx = context.Background()
	}
	r.runOnce(ctx, time.Now())
	if ctx.Err() != nil {
		return
	}
	timer := time.NewTimer(r.nextDelay())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.runOnce(ctx, time.Now())
			if ctx.Err() != nil {
				return
			}
			timer.Reset(r.nextDelay())
		}
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) nextDelay() time.Duration {
	if r == nil {
		return groupUpstreamBalanceRefreshScanInterval
	}
	return nextGroupUpstreamBalanceRefreshDelay(groupUpstreamBalanceRefreshScanInterval, r.randInt64)
}

func nextGroupUpstreamBalanceRefreshDelay(interval time.Duration, randInt64 func(int64) int64) time.Duration {
	if interval <= 0 || randInt64 == nil {
		return interval
	}
	jitter := interval * groupUpstreamBalanceRefreshJitterPercent / 100
	if jitter <= 0 {
		return interval
	}
	span := int64(2*jitter) + 1
	draw := randInt64(span) % span
	if draw < 0 {
		draw += span
	}
	return interval + time.Duration(draw-int64(jitter))
}

func (r *GroupUpstreamBalanceRefreshRunner) runOnce(ctx context.Context, now time.Time) {
	if r == nil || r.groupRepo == nil || r.accountRepo == nil || r.refresher == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	cycleCtx, cancel := context.WithTimeout(ctx, groupUpstreamBalanceRefreshMaxCycleRuntime)
	defer cancel()
	releaseLeader, acquired := tryAcquireSingletonLeaderLock(
		cycleCtx,
		r.lockCache,
		r.db,
		groupUpstreamBalanceRefreshLeaderLockKey,
		r.owner,
		groupUpstreamBalanceRefreshLeaderLockTTL,
	)
	if !acquired {
		return
	}
	defer releaseLeader()
	ctx = cycleCtx

	groups, err := r.groupRepo.ListUpstreamBalanceRefreshEnabled(ctx)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_groups_failed", "error", err)
		return
	}
	r.loadDistributedLastRuns(ctx, groups)
	plans := make(map[int64]*groupUpstreamBalanceRefreshPlanItem)
	accountOrder := make([]int64, 0)
	groupStates := make(map[int64]*groupUpstreamBalanceRefreshGroupState, len(groups))
	for i := range groups {
		if ctx.Err() != nil {
			return
		}
		group := groups[i]
		interval := time.Duration(group.UpstreamBalanceRefreshIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Duration(DefaultUpstreamBalanceRefreshIntervalSeconds) * time.Second
		}
		if lastRunAt, ok := r.lastRun[group.ID]; ok && now.Sub(lastRunAt) < interval {
			continue
		}
		membershipCount, completed, stopBatch := r.addGroupToRefreshPlan(ctx, group, plans, &accountOrder)
		state := &groupUpstreamBalanceRefreshGroupState{pending: membershipCount, failed: !completed}
		groupStates[group.ID] = state
		if completed && membershipCount == 0 {
			r.commitGroupLastRun(ctx, group.ID, now)
		}
		if stopBatch {
			return
		}
	}

	for _, accountID := range accountOrder {
		if ctx.Err() != nil {
			return
		}
		plan := plans[accountID]
		membershipResults, stopBatch := r.refreshAccount(ctx, plan, now)
		for _, result := range membershipResults {
			state := groupStates[result.groupID]
			if state == nil || state.pending <= 0 {
				continue
			}
			state.pending--
			state.failed = state.failed || result.failed
			if state.pending == 0 && !state.failed {
				r.commitGroupLastRun(ctx, result.groupID, now)
			}
		}
		if stopBatch {
			return
		}
	}
}

func groupUpstreamBalanceRefreshLastRunKey(groupID int64) string {
	return groupUpstreamBalanceRefreshLastRunPrefix + strconv.FormatInt(groupID, 10)
}

func (r *GroupUpstreamBalanceRefreshRunner) loadDistributedLastRuns(ctx context.Context, groups []Group) {
	if r.stateRepo == nil || len(groups) == 0 {
		return
	}
	keys := make([]string, 0, len(groups))
	groupByKey := make(map[string]int64, len(groups))
	for i := range groups {
		key := groupUpstreamBalanceRefreshLastRunKey(groups[i].ID)
		if _, exists := groupByKey[key]; exists {
			continue
		}
		keys = append(keys, key)
		groupByKey[key] = groups[i].ID
	}
	values, err := r.stateRepo.GetMultiple(ctx, keys)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.load_last_runs_failed", "error", err)
		return
	}
	for key, value := range values {
		groupID, ok := groupByKey[key]
		if !ok {
			continue
		}
		unixSeconds, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			slog.Warn("group_upstream_balance_refresh.invalid_last_run", "group_id", groupID, "error", err)
			continue
		}
		distributed := time.Unix(unixSeconds, 0).UTC()
		if local, ok := r.lastRun[groupID]; !ok || distributed.After(local) {
			r.lastRun[groupID] = distributed
		}
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) commitGroupLastRun(ctx context.Context, groupID int64, now time.Time) {
	r.lastRun[groupID] = now
	if r.stateRepo == nil {
		return
	}
	if err := r.stateRepo.Set(ctx, groupUpstreamBalanceRefreshLastRunKey(groupID), strconv.FormatInt(now.UTC().Unix(), 10)); err != nil {
		slog.Warn("group_upstream_balance_refresh.store_last_run_failed", "group_id", groupID, "error", err)
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) addGroupToRefreshPlan(
	ctx context.Context,
	group Group,
	plans map[int64]*groupUpstreamBalanceRefreshPlanItem,
	accountOrder *[]int64,
) (membershipCount int, completed bool, stopBatch bool) {
	completed = true
	defer func() {
		if recovered := recover(); recovered != nil {
			membershipCount = 0
			completed = false
			stopBatch = false
			slog.Error(
				"group_upstream_balance_refresh.group_panic",
				"group_id", group.ID,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	if ctx.Err() != nil {
		return 0, false, true
	}
	accounts, err := r.accountRepo.ListUpstreamBalanceRefreshCandidatesByGroupID(ctx, group.ID, groupUpstreamBalanceRefreshCandidateLimit)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_accounts_failed", "group_id", group.ID, "error", err)
		return 0, true, false
	}
	for i := range accounts {
		accountID := accounts[i].ID
		plan := plans[accountID]
		if plan == nil {
			plan = &groupUpstreamBalanceRefreshPlanItem{accountID: accountID}
			plans[accountID] = plan
			*accountOrder = append(*accountOrder, accountID)
		}
		if !groupRefreshPlanContainsGroup(plan.groups, group.ID) {
			plan.groups = append(plan.groups, group)
			membershipCount++
		}
	}
	return membershipCount, true, false
}

func groupRefreshPlanContainsGroup(groups []Group, groupID int64) bool {
	for i := range groups {
		if groups[i].ID == groupID {
			return true
		}
	}
	return false
}

func (r *GroupUpstreamBalanceRefreshRunner) refreshAccount(ctx context.Context, plan *groupUpstreamBalanceRefreshPlanItem, now time.Time) (membershipResults []groupUpstreamBalanceRefreshMembershipResult, stopBatch bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			membershipResults = membershipResults[:0]
			for i := range plan.groups {
				membershipResults = append(membershipResults, groupUpstreamBalanceRefreshMembershipResult{
					groupID: plan.groups[i].ID,
					failed:  true,
				})
			}
			stopBatch = false
			slog.Error(
				"group_upstream_balance_refresh.account_panic",
				"account_id", plan.accountID,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	if ctx.Err() != nil {
		return nil, true
	}
	refreshed, err := r.refresher.Refresh(ctx, plan.accountID)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.refresh_failed", "account_id", plan.accountID, "error", err)
		return terminalGroupMembershipResults(plan.groups), false
	}
	if refreshed == nil {
		return terminalGroupMembershipResults(plan.groups), false
	}
	for i := range plan.groups {
		if ctx.Err() != nil {
			return membershipResults, true
		}
		group := plan.groups[i]
		failed, stopAccount, stopBatch := r.applyPriceGuardMembership(ctx, refreshed, group, now)
		if stopBatch {
			return membershipResults, true
		}
		membershipResults = append(membershipResults, groupUpstreamBalanceRefreshMembershipResult{groupID: group.ID, failed: failed})
		if stopAccount {
			for j := i + 1; j < len(plan.groups); j++ {
				membershipResults = append(membershipResults, groupUpstreamBalanceRefreshMembershipResult{
					groupID: plan.groups[j].ID,
					failed:  true,
				})
			}
			return membershipResults, false
		}
	}
	return membershipResults, false
}

func terminalGroupMembershipResults(groups []Group) []groupUpstreamBalanceRefreshMembershipResult {
	results := make([]groupUpstreamBalanceRefreshMembershipResult, 0, len(groups))
	for i := range groups {
		results = append(results, groupUpstreamBalanceRefreshMembershipResult{groupID: groups[i].ID})
	}
	return results
}

func (r *GroupUpstreamBalanceRefreshRunner) applyPriceGuardMembership(ctx context.Context, account *Account, group Group, now time.Time) (failed bool, stopAccount bool, stopBatch bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			failed = true
			stopBatch = ctx.Err() != nil
			if !stopBatch {
				stopAccount = !r.reloadAccountGuardState(ctx, account)
			}
			slog.Error(
				"group_upstream_balance_refresh.price_guard_panic",
				"group_id", group.ID,
				"account_id", account.ID,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	if err := ApplyGroupUpstreamPriceGuard(ctx, r.accountRepo, account, group, now); err != nil {
		slog.Warn("group_upstream_balance_refresh.price_guard_failed", "group_id", group.ID, "account_id", account.ID, "error", err)
	}
	if ctx.Err() != nil {
		return false, false, true
	}
	if !r.reloadAccountGuardState(ctx, account) {
		return true, true, false
	}
	return false, false, false
}

func (r *GroupUpstreamBalanceRefreshRunner) reloadAccountGuardState(ctx context.Context, account *Account) (trusted bool) {
	if account == nil || r.accountRepo == nil {
		return false
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			trusted = false
			slog.Error(
				"group_upstream_balance_refresh.reload_guard_state_panic",
				"account_id", account.ID,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	latest, err := r.accountRepo.GetByID(ctx, account.ID)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.reload_guard_state_failed", "account_id", account.ID, "error", err)
		return false
	}
	if latest == nil {
		slog.Warn("group_upstream_balance_refresh.reload_guard_state_missing", "account_id", account.ID)
		return false
	}
	account.TempUnschedulableReason = latest.TempUnschedulableReason
	account.TempUnschedulableUntil = latest.TempUnschedulableUntil
	return true
}
