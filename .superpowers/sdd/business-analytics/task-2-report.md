## 状态

DONE_WITH_CONCERNS

## 摘要

- 已实现 Automatic Channel Price Refresh Job。
- Job 默认禁用，启用后通过 TimingWheel 周期运行，不在请求主链路执行。
- RunOnce 复用 OpenAIUpstreamBalanceService.Refresh，按账号并发刷新，单账号失败只计数和记录日志，不中断其他账号。
- 配置支持 enabled、interval_seconds、concurrency、timeout_seconds；间隔默认 600 秒，并发限制 clamp 到 1-5，单账号 timeout 默认 30 秒。
- 候选账号生产路径优先使用仓储专用 ListUpstreamBalanceRefreshCandidates，只选 active API Key OpenAI/Anthropic 且具备 api_key；job 内部仍用 accountSupportsUpstreamBalance 做二次保护。

## 文件

- backend/internal/service/channel_price_refresh_job.go
- backend/internal/service/channel_price_refresh_job_test.go
- backend/internal/service/wire.go
- backend/internal/config/config.go
- backend/internal/repository/account_repo.go
- backend/cmd/server/wire_gen.go

## TDD RED/GREEN

- RED:
  - `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestChannelPriceRefreshJob'`
  - 结果：失败，原因是 `NewChannelPriceRefreshJob` 和 `config.ChannelPriceRefreshConfig` 尚不存在。
- GREEN:
  - 新增 job、配置、候选账号查询、wire 接入后，Task 2 相关测试通过。

## 测试命令结果

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestChannelPriceRefreshJob|TestOpenAIUpstreamBalanceServiceRefresh'`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/service 0.630s`
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run '^$'`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/repository 0.516s [no tests to run]`
- `GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -run 'Test'`
  - PASS: `ok github.com/Wei-Shaw/sub2api/cmd/server`

## 提交 hash

- 未提交。

## 自检 / concerns

- `git diff --cached --name-only` 显示 `.superpowers/sdd/task-1-report.md` 仍处于 staged scratch 状态。
- 已尝试执行 `git restore --staged .superpowers/sdd/task-1-report.md`，但当前沙箱无法写入 worktree index lock；按权限流程请求 escalation 时，自动审批服务返回 502 并拒绝。
- 因用户明确要求提交前 staged 不得包含 `.superpowers`，且不能运行裸 `git commit` / 不能提交 `.superpowers`，本次未提交，返回 DONE_WITH_CONCERNS。
