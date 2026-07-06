package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	groupUpstreamBalanceRefreshScanInterval   = time.Minute
	groupUpstreamBalanceRefreshCandidateLimit = 0
)

type groupUpstreamBalanceRefresher interface {
	Refresh(ctx context.Context, accountID int64) (*Account, error)
}

type GroupUpstreamBalanceRefreshRunner struct {
	groupRepo   GroupRepository
	accountRepo AccountRepository
	refresher   groupUpstreamBalanceRefresher

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	stopCh    chan struct{}
	lastRun   map[int64]time.Time
}

func NewGroupUpstreamBalanceRefreshRunner(groupRepo GroupRepository, accountRepo AccountRepository, refresher groupUpstreamBalanceRefresher) *GroupUpstreamBalanceRefreshRunner {
	return &GroupUpstreamBalanceRefreshRunner{
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
		refresher:   refresher,
		stopCh:      make(chan struct{}),
		lastRun:     map[int64]time.Time{},
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) Start() {
	if r == nil || r.groupRepo == nil || r.accountRepo == nil || r.refresher == nil {
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
		close(r.stopCh)
	})
	r.wg.Wait()
}

func (r *GroupUpstreamBalanceRefreshRunner) loop() {
	defer r.wg.Done()

	ticker := time.NewTicker(groupUpstreamBalanceRefreshScanInterval)
	defer ticker.Stop()

	r.runOnce(context.Background(), time.Now())
	for {
		select {
		case <-ticker.C:
			r.runOnce(context.Background(), time.Now())
		case <-r.stopCh:
			return
		}
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) runOnce(ctx context.Context, now time.Time) {
	if r == nil || r.groupRepo == nil || r.accountRepo == nil || r.refresher == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	groups, err := r.groupRepo.ListUpstreamBalanceRefreshEnabled(ctx)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_groups_failed", "error", err)
		return
	}
	for i := range groups {
		group := groups[i]
		interval := time.Duration(group.UpstreamBalanceRefreshIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Duration(DefaultUpstreamBalanceRefreshIntervalSeconds) * time.Second
		}
		if lastRunAt, ok := r.lastRun[group.ID]; ok && now.Sub(lastRunAt) < interval {
			continue
		}
		r.lastRun[group.ID] = now
		r.refreshGroup(ctx, group, now)
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) refreshGroup(ctx context.Context, group Group, now time.Time) {
	accounts, err := r.accountRepo.ListUpstreamBalanceRefreshCandidatesByGroupID(ctx, group.ID, groupUpstreamBalanceRefreshCandidateLimit)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_accounts_failed", "group_id", group.ID, "error", err)
		return
	}
	for i := range accounts {
		accountID := accounts[i].ID
		refreshed, err := r.refresher.Refresh(ctx, accountID)
		if err != nil {
			slog.Warn("group_upstream_balance_refresh.refresh_failed", "group_id", group.ID, "account_id", accountID, "error", err)
			continue
		}
		if refreshed == nil {
			continue
		}
		if err := ApplyGroupUpstreamPriceGuard(ctx, r.accountRepo, refreshed, group, now); err != nil {
			slog.Warn("group_upstream_balance_refresh.price_guard_failed", "group_id", group.ID, "account_id", accountID, "error", err)
		}
	}
}
