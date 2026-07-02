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
