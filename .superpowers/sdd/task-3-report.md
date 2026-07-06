# Task 3 Report

## What you implemented

- 新增 `GroupUpstreamBalanceRefreshRunner`，按分组扫描开启了上游余额刷新的活跃分组。
- 为每个分组按 `UpstreamBalanceRefreshIntervalSeconds` 做本地节流；未配置时回落到 `DefaultUpstreamBalanceRefreshIntervalSeconds`。
- 每轮对当前分组下支持上游余额刷新的账号执行 `OpenAIUpstreamBalanceService.Refresh(ctx, accountID)`。
- 刷新完成后调用 `ApplyGroupUpstreamPriceGuard(ctx, repo, account, group, now)`，把 Task 2 的价格保护策略接上。
- 新增账号仓储方法 `ListUpstreamBalanceRefreshCandidatesByGroupID`，按分组筛选 active / apikey / openai|anthropic / 含 `api_key` 的账号，并复用 `GetByIDs` 保持 hydration 一致。
- 新增分组仓储方法 `ListUpstreamBalanceRefreshEnabled`，只返回开启了 `upstream_balance_refresh_enabled` 的活跃分组，按 `sort_order, id` 排序。

## Files changed

- `backend/internal/service/group_upstream_balance_refresh_runner.go`
- `backend/internal/service/group_upstream_balance_refresh_runner_test.go`
- `backend/internal/service/group_upstream_balance_refresh_runner_compat_test.go`
- `backend/internal/service/account_service.go`
- `backend/internal/service/group_service.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/account_repo_checkin_candidates_test.go`
- `backend/internal/repository/group_repo.go`

## TDD Evidence

### RED

Command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'TestGroupUpstreamBalanceRefreshRunner|TestListUpstreamBalanceRefreshCandidatesByGroupID'
```

Relevant failing output before implementation:

```text
# github.com/Wei-Shaw/sub2api/internal/repository [github.com/Wei-Shaw/sub2api/internal/repository.test]
internal/repository/account_repo_checkin_candidates_test.go:51:24: repo.ListUpstreamBalanceRefreshCandidatesByGroupID undefined (type *accountRepository has no field or method ListUpstreamBalanceRefreshCandidatesByGroupID)
# github.com/Wei-Shaw/sub2api/internal/service [github.com/Wei-Shaw/sub2api/internal/service.test]
internal/service/group_upstream_balance_refresh_runner_test.go:75:12: undefined: NewGroupUpstreamBalanceRefreshRunner
internal/service/group_upstream_balance_refresh_runner_test.go:97:12: undefined: NewGroupUpstreamBalanceRefreshRunner
FAIL
```

Why failure was expected:

- RED 阶段先写了 runner 测试和 repository 测试，此时生产代码与接口都还不存在，所以应当以“缺少方法/构造器”失败。

### GREEN

Command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'TestGroupUpstreamBalanceRefreshRunner|TestListUpstreamBalanceRefreshCandidatesByGroupID'
```

Passing output after implementation:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	3.307s
ok  	github.com/Wei-Shaw/sub2api/internal/repository	(cached)
```

## Self-review findings

- runner 只负责分组扫描、按组限频、逐账号 refresh 和价格 guard，不提前做 Task 4 的 wire/startup 接入。
- 仓储筛选条件与 brief 对齐，且复用了 `GetByIDs`，避免返回轻量 SQL 结果时丢失现有 hydration 逻辑。
- 由于 `AccountRepository` / `GroupRepository` 新增方法，补了一个很小的 `group_upstream_balance_refresh_runner_compat_test.go` 来给默认测试构建里的旧 stub 补空实现，避免扩大无关测试改动面。

## Concerns, if any

- `group_upstream_balance_refresh_runner_compat_test.go` 只是为默认测试构建补齐接口方法集，不影响生产行为；后续如果仓库继续给这些大接口加方法，测试桩维护成本会继续增长。
