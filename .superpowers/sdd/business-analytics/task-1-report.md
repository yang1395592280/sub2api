# Task 1 Report: Usage Channel Price Snapshot

## 1. 状态

DONE

## 2. 改动摘要

- 为 `usage_logs` 增加渠道价格快照字段：`channel_price_snapshot`、`channel_price_source`、`channel_price_refreshed_at`。
- 更新 Ent schema、生成代码和 SQL migration `158_business_analytics.sql`。
- 扩展 `service.UsageLog`、repository insert/select/scan、single/batch/best-effort 写入路径，保持 nil 值为 NULL。
- 在普通 Gateway 与 OpenAI Gateway 的 usage log 构造后，从 selected account 的 `ChannelPrice` 和 `Extra` 轻量写入快照；未触发任何自动刷新。
- 增加 repository 与 service 聚焦测试覆盖快照写入和读取。

## 3. 修改文件列表

- `backend/ent/schema/usage_log.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/usagelog.go`
- `backend/ent/usagelog/usagelog.go`
- `backend/ent/usagelog/where.go`
- `backend/ent/usagelog_create.go`
- `backend/ent/usagelog_update.go`
- `backend/migrations/158_business_analytics.sql`
- `backend/internal/service/usage_log.go`
- `backend/internal/service/usage_log_helpers.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/repository/usage_log_repo_request_type_test.go`
- `backend/internal/service/gateway_record_usage_test.go`
- `backend/internal/service/openai_gateway_record_usage_test.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`

## 4. 测试命令和结果

- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/repository -run 'UsageLog.*ChannelPriceSnapshot|UsageLogRepository.*ChannelPriceSnapshot'`
  - 结果：PASS，`ok github.com/Wei-Shaw/sub2api/internal/repository 0.500s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service -run 'RecordUsage|ChannelPriceSnapshot'`
  - 结果：PASS，`ok github.com/Wei-Shaw/sub2api/internal/service 0.557s`

## 5. 提交 hash

7550239a

## 6. 自检结论和 concerns

- 自检结论：Task 1 brief 范围内的 usage_logs 快照字段、service model、repository insert/select、请求落 usage 时从 selected account 读取快照均已完成。
- 未改动用户扣费逻辑、账号调度逻辑，未在请求主链路调用渠道价格刷新。
- 缺少价格时 helper 直接返回 nil，不会写 0。
- Concern：工作区进入任务前已有 `.superpowers/sdd/progress.md` 未提交改动，本任务未修改、未 stage、未提交该文件。

---

## Fix worker follow-up: 2026-07-01 12:56:02 CST

### 修复摘要

- 补齐 `TestScanUsageLogRequestTypeAndLegacyFallback` 内 4 个 scan stub 的 `channel_price_snapshot`、`channel_price_source`、`channel_price_refreshed_at` 列值，保持与 `usageLogSelectColumns` 54 列一致。
- 在 `TestPrepareUsageLogInsert_PersistsChannelPriceSnapshot` 增加 `prepared.args` 与 `usageLogInsertArgTypes` 的长度错位保护。
- 新增 integration 用例 `TestUsageLogRepoSuite/TestCreateAndGetByID_ReturnsChannelPriceSnapshot`，覆盖通过 repository `Create` 真实写入 channel price snapshot 字段后再用 `GetByID` 读回。

### 测试命令和结果

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestScanUsageLogRequestTypeAndLegacyFallback|ChannelPriceSnapshot'`
  - 结果：PASS，`ok github.com/Wei-Shaw/sub2api/internal/repository 0.490s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/repository -run 'UsageLog.*ChannelPriceSnapshot|UsageLogRepository.*ChannelPriceSnapshot'`
  - 结果：PASS，`ok github.com/Wei-Shaw/sub2api/internal/repository 0.572s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 -tags=integration ./internal/repository -run 'TestUsageLogRepoSuite/TestCreateAndGetByID_ReturnsChannelPriceSnapshot'`
  - 结果：PASS，`ok github.com/Wei-Shaw/sub2api/internal/repository 0.920s`

### 提交 hash

- 025a24a49ac04772f03349479f4cf843f3ae447b

### concerns

- `.superpowers/sdd/task-1-report.md` 在进入本次修复前已处于 staged 状态；本次不会提交 `.superpowers` scratch 文件。
