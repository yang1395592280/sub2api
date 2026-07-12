# Group Usage Stale Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让分组用量缓存超过 5 分钟后立即返回最近成功结果，并在后台单飞刷新，避免页面周期性等待全历史聚合。

**Architecture:** 保留现有 API、handler、repository SQL 和前端行为，仅重构 `DashboardService` 的进程内缓存状态机。缓存条目记录成功更新时间与最近刷新尝试时间，`singleflight.Group` 同时合并 cold miss 和过期缓存后台刷新；cold miss 共享查询使用独立 service 级 30 秒超时 context，各请求只控制自己的等待。缓存按精确 `todayStart` key 保存距最新 key 48 小时内的多时区结果；失败后保留旧值并进入 30 秒冷却。

**Tech Stack:** Go 1.26.5、`golang.org/x/sync/singleflight`、`testify/require`

## Global Constraints

- 缓存新鲜期固定为 5 分钟。
- 后台刷新超时和失败重试冷却均固定为 30 秒。
- 多时区精确 `todayStart` key 固定滚动保留 48 小时，不新增配置。
- 不新增数据库迁移、预聚合表、依赖或配置项。
- 不修改 `/api/v1/admin/groups/usage-summary` 的请求参数与响应结构。
- 不修改 `GetAllGroupUsageSummary` SQL、统计口径或前端展示。
- 所有返回切片都必须是缓存数据的防御性复制。

---

### Task 1: 过期可读缓存与并发单飞

**Files:**
- Modify: `backend/internal/service/dashboard_service.go`
- Test: `backend/internal/service/dashboard_service_test.go`

**Interfaces:**
- Consumes: `UsageLogRepository.GetAllGroupUsageSummary(context.Context, time.Time) ([]usagestats.GroupUsageSummary, error)`
- Produces: 保持 `DashboardService.GetGroupUsageSummary(context.Context, time.Time) ([]usagestats.GroupUsageSummary, error)` 签名不变；新增私有方法 `loadGroupUsageSummary`、`refreshGroupUsageSummaryAsync`、`storeGroupUsageSummary`。

- [ ] **Step 1: 扩展测试仓储，使并发测试可以控制查询开始和结束**

在 `usageRepoStub` 增加函数钩子：

```go
groupUsageSummaryFn func(ctx context.Context, todayStart time.Time, call int32) ([]usagestats.GroupUsageSummary, error)
```

并将 stub 方法改为：

```go
func (s *usageRepoStub) GetAllGroupUsageSummary(ctx context.Context, todayStart time.Time) ([]usagestats.GroupUsageSummary, error) {
	call := atomic.AddInt32(&s.groupUsageSummaryCall, 1)
	if s.groupUsageSummaryFn != nil {
		return s.groupUsageSummaryFn(ctx, todayStart, call)
	}
	if s.groupUsageSummaryErr != nil {
		return nil, s.groupUsageSummaryErr
	}
	if len(s.groupUsageSummaries) == 0 {
		return nil, nil
	}
	idx := int(call - 1)
	if idx >= len(s.groupUsageSummaries) {
		idx = len(s.groupUsageSummaries) - 1
	}
	return s.groupUsageSummaries[idx], nil
}
```

- [ ] **Step 2: 写过期缓存立即返回并后台更新的失败测试**

新增 `TestDashboardService_GroupUsageSummaryServesStaleWhileRefreshing`。测试先直接放入过期条目，仓储查询通过 channel 阻塞；调用方法后必须在释放仓储前拿到旧值，再释放查询并等待缓存变为新值：

```go
func TestDashboardService_GroupUsageSummaryServesStaleWhileRefreshing(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	var startedOnce sync.Once
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(ctx context.Context, _ time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}},
		expiresAt: time.Now().Add(-time.Minute),
	}

	type result struct {
		value []usagestats.GroupUsageSummary
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		value, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
		resultCh <- result{value: value, err: err}
	}()
	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}}, got.value)
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("stale cache request blocked on refresh")
	}
	<-started
	close(release)
	require.Eventually(t, func() bool {
		svc.groupUsageSummaryMu.RLock()
		defer svc.groupUsageSummaryMu.RUnlock()
		return svc.groupUsageSummaryCache[key].results[0].TotalCost == 12
	}, time.Second, 10*time.Millisecond)
}
```

- [ ] **Step 3: 写冷缓存并发请求只查一次的失败测试**

新增以下测试；每个 goroutine 把结果写入 channel，主测试统一断言，避免在 goroutine 内调用 `require`：

```go
func TestDashboardService_GroupUsageSummaryColdMissUsesSingleflight(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	var startedOnce sync.Once
	release := make(chan struct{})
	repo := &usageRepoStub{groupUsageSummaryFn: func(ctx context.Context, _ time.Time, _ int32) ([]usagestats.GroupUsageSummary, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
			return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := NewDashboardService(repo, nil, nil, nil)
	type result struct {
		value []usagestats.GroupUsageSummary
		err   error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	for range 8 {
		go func() {
			<-start
			value, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	<-started
	close(release)
	for range 8 {
		got := <-results
		require.NoError(t, got.err)
		require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 3, TotalCost: 12}}, got.value)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.groupUsageSummaryCall))
}
```

- [ ] **Step 4: 运行新测试并确认 RED**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GroupUsageSummary(ServesStaleWhileRefreshing|ColdMissUsesSingleflight)' -count=1
```

Expected: FAIL，原因是过期命中仍同步等待查询，冷缓存也未使用 `singleflight`。先确认失败来自目标行为，而不是测试语法或 stub 错误。

- [ ] **Step 5: 实现缓存状态与单飞回源**

在 imports 增加：

```go
"golang.org/x/sync/singleflight"
```

增加常量和状态：

```go
const (
	defaultGroupUsageSummaryCacheTTL        = 5 * time.Minute
	defaultGroupUsageSummaryRefreshTimeout = 30 * time.Second
)

type groupUsageSummaryCacheEntry struct {
	results   []usagestats.GroupUsageSummary
	updatedAt time.Time
}
```

在 `DashboardService` 增加：

```go
groupUsageSummarySF             singleflight.Group
groupUsageSummaryRefreshTimeout time.Duration
```

构造函数增加：

```go
groupUsageSummaryCache:          make(map[int64]groupUsageSummaryCacheEntry),
groupUsageSummaryCacheTTL:       defaultGroupUsageSummaryCacheTTL,
groupUsageSummaryRefreshTimeout: defaultGroupUsageSummaryRefreshTimeout,
```

将 `GetGroupUsageSummary` 与缓存辅助方法替换为以下状态机；`fmt` 已被当前文件导入，可用于 singleflight key：

```go
func (s *DashboardService) GetGroupUsageSummary(ctx context.Context, todayStart time.Time) ([]usagestats.GroupUsageSummary, error) {
	key := groupUsageSummaryCacheKey(todayStart)
	now := time.Now()

	s.groupUsageSummaryMu.RLock()
	entry, ok := s.groupUsageSummaryCache[key]
	s.groupUsageSummaryMu.RUnlock()
	if ok {
		results := cloneGroupUsageSummary(entry.results)
		if now.Sub(entry.updatedAt) > s.groupUsageSummaryCacheTTL {
			s.refreshGroupUsageSummaryAsync(key, todayStart, now)
		}
		return results, nil
	}

	return s.loadGroupUsageSummary(ctx, key, todayStart)
}

func (s *DashboardService) loadGroupUsageSummary(ctx context.Context, key int64, todayStart time.Time) ([]usagestats.GroupUsageSummary, error) {
	resultCh := s.groupUsageSummarySF.DoChan(fmt.Sprintf("%d", key), func() (any, error) {
		s.groupUsageSummaryMu.RLock()
		entry, ok := s.groupUsageSummaryCache[key]
		s.groupUsageSummaryMu.RUnlock()
		if ok {
			return cloneGroupUsageSummary(entry.results), nil
		}
		loadCtx, cancel := context.WithTimeout(context.Background(), s.groupUsageSummaryRefreshTimeout)
		defer cancel()
		results, err := s.usageRepo.GetAllGroupUsageSummary(loadCtx, todayStart)
		if err != nil {
			return nil, fmt.Errorf("get group usage summary: %w", err)
		}
		s.storeGroupUsageSummary(key, results, time.Now())
		return cloneGroupUsageSummary(results), nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		value, ok := result.Val.([]usagestats.GroupUsageSummary)
		if !ok {
			return nil, fmt.Errorf("get group usage summary: unexpected singleflight result type %T", result.Val)
		}
		return cloneGroupUsageSummary(value), nil
	}
}

func (s *DashboardService) refreshGroupUsageSummaryAsync(key int64, todayStart, now time.Time) {
	s.groupUsageSummarySF.DoChan(fmt.Sprintf("%d", key), func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), s.groupUsageSummaryRefreshTimeout)
		defer cancel()
		results, err := s.usageRepo.GetAllGroupUsageSummary(ctx, todayStart)
		if err != nil {
			logger.LegacyPrintf("service.dashboard", "[Dashboard] 分组用量缓存异步刷新失败: today_start=%s err=%v", todayStart.Format(time.RFC3339), err)
			return nil, err
		}
		s.storeGroupUsageSummary(key, results, time.Now())
		return cloneGroupUsageSummary(results), nil
	})
}

func (s *DashboardService) storeGroupUsageSummary(key int64, results []usagestats.GroupUsageSummary, now time.Time) {
	s.groupUsageSummaryMu.Lock()
	defer s.groupUsageSummaryMu.Unlock()
	s.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   cloneGroupUsageSummary(results),
		updatedAt: now,
	}
}
```

删除旧的 `expiresAt`、`getCachedGroupUsageSummary` 和 `setCachedGroupUsageSummary` 实现，并把 Step 2 的测试夹具从 `expiresAt` 更新为 `updatedAt: time.Now().Add(-6 * time.Minute)`。

- [ ] **Step 6: 运行 Task 1 测试并确认 GREEN**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GroupUsageSummary' -count=1
```

Expected: PASS；现有五分钟缓存、防御性复制、跨日期测试和新增并发测试全部通过。

- [ ] **Step 7: 运行竞态检查**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestDashboardService_GroupUsageSummary' -count=1
```

Expected: PASS，且无 `DATA RACE`。

- [ ] **Step 8: 提交核心缓存实现**

```bash
git add backend/internal/service/dashboard_service.go backend/internal/service/dashboard_service_test.go
git commit -m "perf: serve stale group usage while refreshing"
```

---

### Task 2: 刷新失败冷却与 48 小时多时区滚动保留

**Files:**
- Modify: `backend/internal/service/dashboard_service.go`
- Test: `backend/internal/service/dashboard_service_test.go`

**Interfaces:**
- Consumes: Task 1 的 `groupUsageSummaryCacheEntry`、`refreshGroupUsageSummaryAsync` 和 `storeGroupUsageSummary`。
- Produces: 失败后 30 秒内不重复查询；成功写入后保留距最新精确 key 不超过 48 小时的多个缓存，拒绝窗口外晚到结果。

- [ ] **Step 1: 写失败冷却的失败测试**

新增：

```go
func TestDashboardService_GroupUsageSummaryRefreshCooldown(t *testing.T) {
	todayStart := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	repo := &usageRepoStub{groupUsageSummaryErr: errors.New("refresh failed")}
	svc := NewDashboardService(repo, nil, nil, nil)
	key := groupUsageSummaryCacheKey(todayStart)
	svc.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:   []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 10}},
		updatedAt: time.Now().Add(-6 * time.Minute),
	}

	got, err := svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Equal(t, float64(10), got[0].TotalCost)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&repo.groupUsageSummaryCall) == 1
	}, time.Second, 10*time.Millisecond)
	_, err = svc.GetGroupUsageSummary(context.Background(), todayStart)
	require.NoError(t, err)
	require.Never(t, func() bool {
		return atomic.LoadInt32(&repo.groupUsageSummaryCall) > 1
	}, 100*time.Millisecond, 10*time.Millisecond)
}
```

- [ ] **Step 2: 强化多时区并存和滚动清理测试**

在 `TestDashboardService_GroupUsageSummaryCacheSeparatesTodayStart` 中验证两个相邻时区边界 key 同时存在，重复读取不回源；另补 48 小时边界保留、窗口外清理，以及旧 key 晚完成不驱逐新 key 的测试。

```go
svc.groupUsageSummaryMu.RLock()
defer svc.groupUsageSummaryMu.RUnlock()
require.Len(t, svc.groupUsageSummaryCache, 2)
require.Contains(t, svc.groupUsageSummaryCache, groupUsageSummaryCacheKey(firstDay))
require.Contains(t, svc.groupUsageSummaryCache, groupUsageSummaryCacheKey(secondDay))
```

- [ ] **Step 3: 运行新增测试并确认 RED**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GroupUsageSummary(RefreshCooldown|CacheSeparatesTodayStart)' -count=1
```

Expected: FAIL，旧实现会在首次刷新错误时把错误同步返回，并且过期请求会重复触发仓储查询；单日期清理还会驱逐 48 小时窗口内的另一个时区 key。

- [ ] **Step 4: 实现失败冷却和 48 小时滚动保留**

增加常量、字段和构造函数初始化：

```go
const defaultGroupUsageSummaryRetryCooldown = 30 * time.Second

type groupUsageSummaryCacheEntry struct {
	results            []usagestats.GroupUsageSummary
	updatedAt          time.Time
	lastRefreshAttempt time.Time
}

// DashboardService
groupUsageSummaryRetryCooldown time.Duration
```

将 `refreshGroupUsageSummaryAsync` 改为先记录刷新尝试；冷却期内直接返回，不启动 `DoChan`：

```go
func (s *DashboardService) refreshGroupUsageSummaryAsync(key int64, todayStart, now time.Time) {
	if !s.markGroupUsageSummaryRefreshAttempt(key, now) {
		return
	}
	s.groupUsageSummarySF.DoChan(fmt.Sprintf("%d", key), func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), s.groupUsageSummaryRefreshTimeout)
		defer cancel()
		results, err := s.usageRepo.GetAllGroupUsageSummary(ctx, todayStart)
		if err != nil {
			logger.LegacyPrintf("service.dashboard", "[Dashboard] 分组用量缓存异步刷新失败: today_start=%s err=%v", todayStart.Format(time.RFC3339), err)
			return nil, err
		}
		s.storeGroupUsageSummary(key, results, time.Now())
		return nil, nil
	})
}

func (s *DashboardService) markGroupUsageSummaryRefreshAttempt(key int64, now time.Time) bool {
	s.groupUsageSummaryMu.Lock()
	defer s.groupUsageSummaryMu.Unlock()
	entry, ok := s.groupUsageSummaryCache[key]
	if !ok || (!entry.lastRefreshAttempt.IsZero() && now.Sub(entry.lastRefreshAttempt) < s.groupUsageSummaryRetryCooldown) {
		return false
	}
	entry.lastRefreshAttempt = now
	s.groupUsageSummaryCache[key] = entry
	return true
}
```

构造函数同时增加：

```go
groupUsageSummaryRetryCooldown: defaultGroupUsageSummaryRetryCooldown,
```

将 `storeGroupUsageSummary` 改为成功写入时保留距最新 key 48 小时内（含边界）的精确 key，并记录本次成功尝试时间。若旧结果晚到且已落后最新 key 超过 48 小时，则拒绝写入；窗口内旧结果只能写自己的 key，不得删除较新的有效 key：

```go
func (s *DashboardService) storeGroupUsageSummary(key int64, results []usagestats.GroupUsageSummary, now time.Time) {
	s.groupUsageSummaryMu.Lock()
	defer s.groupUsageSummaryMu.Unlock()
	latestKey := key
	for cachedKey := range s.groupUsageSummaryCache {
		if cachedKey > latestKey {
			latestKey = cachedKey
		}
	}
	retentionNanos := (48 * time.Hour).Nanoseconds()
	if key < latestKey-retentionNanos {
		return
	}
	for cachedKey := range s.groupUsageSummaryCache {
		if cachedKey < latestKey-retentionNanos {
			delete(s.groupUsageSummaryCache, cachedKey)
		}
	}
	s.groupUsageSummaryCache[key] = groupUsageSummaryCacheEntry{
		results:            cloneGroupUsageSummary(results),
		updatedAt:          now,
		lastRefreshAttempt: now,
	}
}
```

刷新失败路径保持旧条目和旧 `updatedAt`，并保留 Task 1 中包含 `today_start` 和底层错误的日志。

- [ ] **Step 5: 运行全部分组用量 Service 测试**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GroupUsageSummary' -count=1
```

Expected: PASS。

- [ ] **Step 6: 运行完整 Service 包与竞态检查**

Run:

```bash
(cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1)
(cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestDashboardService_GroupUsageSummary' -count=1)
```

Expected: 两条命令均 PASS，无 panic、超时或 `DATA RACE`。

- [ ] **Step 7: 格式检查并提交韧性测试**

```bash
gofmt -w backend/internal/service/dashboard_service.go backend/internal/service/dashboard_service_test.go
git diff --check
git add backend/internal/service/dashboard_service.go backend/internal/service/dashboard_service_test.go
git commit -m "test: cover group usage cache refresh failures"
```

---

### Task 3: 最终验证与变更审查

**Files:**
- Verify: `backend/internal/service/dashboard_service.go`
- Verify: `backend/internal/service/dashboard_service_test.go`
- Verify: `docs/superpowers/specs/2026-07-12-group-usage-stale-cache-design.md`

**Interfaces:**
- Consumes: Task 1 和 Task 2 的完整实现。
- Produces: 可交付的验证证据，不新增功能。

- [ ] **Step 1: 检查变更范围**

Run:

```bash
git diff custom-main...HEAD --stat
git diff custom-main...HEAD -- backend/internal/service/dashboard_service.go backend/internal/service/dashboard_service_test.go
```

Expected: 业务改动仅限 DashboardService 分组用量缓存和对应测试；无 API、repository SQL、前端或依赖变更。

- [ ] **Step 2: 运行最终验证**

Run:

```bash
(cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1)
(cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestDashboardService_GroupUsageSummary' -count=1)
git diff --check custom-main...HEAD
```

Expected: 全部命令退出码为 0。

- [ ] **Step 3: 审查提交历史与工作区**

Run:

```bash
git log --oneline custom-main..HEAD
git status --short --branch
```

Expected: 包含设计、计划、核心实现和韧性测试提交；工作区干净，当前分支为 `feature/group-usage-stale-cache`。
