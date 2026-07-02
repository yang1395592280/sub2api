# Task 1 Report

## 任务结论
- 已完成 Task 1 后端 service 层实现：
  - 新增 `Sub2APICheckinService`
  - 落地签到状态、调度、补签、失败重试
  - 复用 sub2api 上游管理鉴权链路
  - 补充相关 service 测试
- 未改前端、未新增表。
- `account_credentials_redact.go` 本次未改：`upstream_admin_access_token` / `upstream_admin_refresh_token` / `upstream_admin_password` 已在现有敏感字段清单内。

## 实际修改文件
- `backend/internal/service/openai_upstream_balance_service.go`
- `backend/internal/service/sub2api_checkin_service.go`
- `backend/internal/service/sub2api_checkin_service_test.go`

## 核心实现说明
- 在 `sub2api_checkin_service.go` 新增：
  - `Sub2APICheckinService`
  - `Start()` / `Stop()` / `RefreshNow()`
  - `isSub2APICheckinEnabled`
  - `checkinWindowForDate`
  - `randomTimeBetween`
  - `parseHHMM`
  - `buildSub2APICheckinURL`
  - `classifySub2APICheckinResponse`
  - 运行状态写回 `extra` 的成功/失败更新逻辑
- 调度行为：
  - 使用 `ListSub2APICheckinCandidates(ctx, limit)` 获取候选账号
  - 使用服务器本地时区（构造器传入，测试使用 `UTC+8`）
  - 当天无计划时生成窗口内随机执行时间
  - 错过窗口且当天未成功时立即补签
  - 失败后按 10-30 分钟随机延迟重试
  - `retry_count` 按本地日期隔离，跨天自动归零
- 鉴权复用：
  - 从 `OpenAIUpstreamBalanceService` 提取共享的 `resolveSub2APIAdminAuthorization`
  - 回退顺序为：`refresh token -> access token -> email/password login`
  - 刷新到的新 token 通过 `BulkUpdate` 回写 `credentials`
- 安全性：
  - 未把敏感凭据写入日志或 `extra`
  - check-in URL 仅允许相对路径或与 base URL 同源的绝对地址

## TDD / 执行记录
1. 先新增 `backend/internal/service/sub2api_checkin_service_test.go`
2. 执行 RED：
   - `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService|TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate' -v`
   - 结果：失败，缺少 `NewSub2APICheckinService` 和签到状态常量
3. 实现 `Sub2APICheckinService` 与共享 sub2api admin 鉴权 helper
4. 执行 `gofmt`
5. 执行 GREEN：
   - `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService|TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate' -v`
   - 结果：通过
6. 扩大相关回归范围：
   - `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIUpstreamBalanceServiceRefresh|TestSub2APICheckinService' -v`
   - 结果：通过

## 执行过的命令与结果
- `gofmt -w backend/internal/service/sub2api_checkin_service.go backend/internal/service/sub2api_checkin_service_test.go backend/internal/service/openai_upstream_balance_service.go`
  - 结果：成功
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService|TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate' -v`
  - 首次结果：失败（RED），缺少新 service / 常量
  - 再次结果：通过
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIUpstreamBalanceServiceRefresh|TestSub2APICheckinService' -v`
  - 结果：通过，`ok github.com/Wei-Shaw/sub2api/internal/service 0.549s`

## 测试覆盖点
- 窗口内计划时间生成
- “今日已签到” 视为成功
- 重试次数按天重置
- sub2api admin 鉴权回退顺序
- upstream balance 既有 sub2api admin 刷新路径回归

## Commit
- Commit message: `feat: add sub2api checkin service`

## Concerns
- 当前只完成 Task 1 要求的后端 service 层与测试；未在 server/wire 中接入后台启动生命周期，这部分应由后续任务按整体集成范围处理。

---

## 2026-07-02 Task 1 review fix follow-up

### 改了什么
- 在 `backend/internal/service/sub2api_checkin_service.go` 增加同日本地日期重试上限判断：当 `upstream_checkin_retry_date` 为当天且 `upstream_checkin_retry_count >= 3` 时，`reconcileAccount()` 直接停止，不再重新生成或补写 `upstream_checkin_next_run_at`。
- 保留跨天行为：只有同一天命中上限才停止调度；本地日期进入下一天后，仍按原逻辑重新规划并在失败时从 `retry_count=1` 开始累计。
- 在 `backend/internal/service/sub2api_checkin_service_test.go` 新增 `TestSub2APICheckinServiceReconcileSkipsSchedulingAfterRetryCapSameDay`，覆盖“同日已到 3 次且 `next_run_at` 为空时，再次 reconcile 不应重新计划/执行”。

### 根因说明
- 现有 `buildFailureUpdates()` 在同日达到最大重试次数后会把 `upstream_checkin_next_run_at` 清空。
- 但 `reconcileAccount()` 之前只按“今天是否成功”和“next_run_at 是否有效/落在窗口内”决定是否重新排程，没有把“今天是否已经达到重试上限”作为停止条件。
- 因此下一轮扫描会把空的 `next_run_at` 视为缺失，再次给当天补一个新时间，破坏“失败每天最多重试 3 次”约束。

### 测试命令和结果
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinServiceReconcileSkipsSchedulingAfterRetryCapSameDay' -v`
  - 首次结果：失败，`reconcileAccount()` 仍然写入新的调度更新
  - 修复后结果：通过
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService|TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate' -v`
  - 结果：通过，相关 check-in 与 sub2api admin 回归均为绿色

---

## 2026-07-02 Task 1 retry-cap follow-up 2

### 改了什么
- 调整 `backend/internal/service/sub2api_checkin_service.go` 的 `reconcileAccount()` 顺序：先解析并判断 `upstream_checkin_next_run_at` 是否还是“同日本地日期的有效既有计划”，再决定当天是否因 `retry_count >= 3` 停止。
- 现在同日 `retry_count == 3` 时分两种处理：
  - 若已有同日本地日期的有效 `next_run_at`，则保留该计划；一旦到点，仍执行这一次已计划的第 3 次重试。
  - 若没有同日本地日期的有效 `next_run_at`，则直接停止当天调度，不再重新规划。
- 保持失败后封顶行为：第 3 次重试失败后，`buildFailureUpdates()` 继续只写回 `retry_count=3` 且清空 `upstream_checkin_next_run_at`，不再补新的当天时间。
- 在 `backend/internal/service/sub2api_checkin_service_test.go` 新增：
  - `TestSub2APICheckinServiceReconcileExecutesPlannedFinalRetryWhenDue`
  - `TestSub2APICheckinServiceFinalRetryFailureDoesNotScheduleNewRun`
- 保留上一轮 `TestSub2APICheckinServiceReconcileSkipsSchedulingAfterRetryCapSameDay`，覆盖“已封顶且无同日有效计划时不再重新计划”。

### 根因说明
- 上一轮把“同日达到上限且没有计划”与“同日达到上限但已经排好了最后一次计划”都归入 `reachedRetryCapForLocalDate()` 的统一短路分支。
- 结果是 `retry_count == 3` 且 `next_run_at` 已经到了时，`reconcileAccount()` 还没来得及比较 `now >= next_run_at`，就先 `return nil`，导致最后一次已计划重试永远不会真的发请求。

### 测试命令和结果
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService(ReconcileExecutesPlannedFinalRetryWhenDue|FinalRetryFailureDoesNotScheduleNewRun|ReconcileSkipsSchedulingAfterRetryCapSameDay)' -v`
  - 首次结果：失败，`TestSub2APICheckinServiceReconcileExecutesPlannedFinalRetryWhenDue` 未发出请求
  - 修复后结果：通过
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSub2APICheckinService|TestOpenAIUpstreamBalanceServiceRefresh_Sub2APIAdminTokenResolvesEffectiveRate' -v`
  - 结果：通过
