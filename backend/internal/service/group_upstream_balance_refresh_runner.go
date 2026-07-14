package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand/v2"
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
)

type groupUpstreamBalanceRefresher interface {
	Refresh(ctx context.Context, accountID int64) (*Account, error)
}

type GroupUpstreamBalanceRefreshRunner struct {
	groupRepo   GroupRepository
	accountRepo AccountRepository
	refresher   groupUpstreamBalanceRefresher
	lockCache   LeaderLockCache
	db          *sql.DB
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
	parentCtx, cancel := context.WithCancel(context.Background())
	return &GroupUpstreamBalanceRefreshRunner{
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
		refresher:   refresher,
		lockCache:   lockCache,
		db:          db,
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
	plans := make(map[int64]*groupUpstreamBalanceRefreshPlanItem)
	accountOrder := make([]int64, 0)
	dueGroups := make([]Group, 0, len(groups))
	completedGroups := make(map[int64]bool, len(groups))
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
		dueGroups = append(dueGroups, group)
		completedGroups[group.ID] = true
		completed, stopBatch := r.addGroupToRefreshPlan(ctx, group, plans, &accountOrder)
		if !completed {
			completedGroups[group.ID] = false
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
		panicked, stopBatch := r.refreshAccount(ctx, plan, now)
		if panicked {
			for i := range plan.groups {
				completedGroups[plan.groups[i].ID] = false
			}
		}
		if stopBatch {
			return
		}
	}
	for i := range dueGroups {
		if completedGroups[dueGroups[i].ID] {
			r.lastRun[dueGroups[i].ID] = now
		}
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) addGroupToRefreshPlan(
	ctx context.Context,
	group Group,
	plans map[int64]*groupUpstreamBalanceRefreshPlanItem,
	accountOrder *[]int64,
) (completed bool, stopBatch bool) {
	completed = true
	defer func() {
		if recovered := recover(); recovered != nil {
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
		return false, true
	}
	accounts, err := r.accountRepo.ListUpstreamBalanceRefreshCandidatesByGroupID(ctx, group.ID, groupUpstreamBalanceRefreshCandidateLimit)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_accounts_failed", "group_id", group.ID, "error", err)
		return true, false
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
		}
	}
	return true, false
}

func groupRefreshPlanContainsGroup(groups []Group, groupID int64) bool {
	for i := range groups {
		if groups[i].ID == groupID {
			return true
		}
	}
	return false
}

func (r *GroupUpstreamBalanceRefreshRunner) refreshAccount(ctx context.Context, plan *groupUpstreamBalanceRefreshPlanItem, now time.Time) (panicked bool, stopBatch bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = true
			stopBatch = false
			slog.Error(
				"group_upstream_balance_refresh.account_panic",
				"account_id", plan.accountID,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	if ctx.Err() != nil {
		return false, true
	}
	refreshed, err := r.refresher.Refresh(ctx, plan.accountID)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.refresh_failed", "account_id", plan.accountID, "error", err)
		return false, false
	}
	if refreshed == nil {
		return false, false
	}
	for i := range plan.groups {
		if ctx.Err() != nil {
			return false, true
		}
		group := plan.groups[i]
		if err := ApplyGroupUpstreamPriceGuard(ctx, r.accountRepo, refreshed, group, now); err != nil {
			slog.Warn("group_upstream_balance_refresh.price_guard_failed", "group_id", group.ID, "account_id", plan.accountID, "error", err)
		}
	}
	return false, false
}
